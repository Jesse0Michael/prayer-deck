// Command seed adds a card, with its artwork, to a deployed deck for every
// image in a directory.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// cardNames maps an image's filename, without extension, to the card's name.
// Images without an entry (the card back, torn-edge art) are not cards.
var cardNames = map[string]string{
	"amanda":                   "Amanda",
	"clementine":               "Clementine",
	"dad":                      "Dad",
	"grandma":                  "Grandma",
	"josh":                     "Josh",
	"keatan":                   "Keatan",
	"kristina":                 "Kristina",
	"lance":                    "Lance",
	"logan":                    "Logan",
	"lorelai":                  "Lorelai",
	"marco":                    "Marco",
	"mom":                      "Mom",
	"nana":                     "Nana",
	"nathan":                   "Nathan",
	"papa":                     "Papa",
	"tim":                      "Tim",
	"andrew_ale":               "Andrew & Ale",
	"brice_angela":             "Brice & Angela",
	"cooper_booker":            "Cooper & Booker",
	"craig_kim":                "Craig & Kim",
	"dean_kristi":              "Dean & Kristi",
	"gpa_diane":                "Gpa & Diane",
	"jacob_molly":              "Jacob & Molly",
	"jessica_lucy":             "Jessica & Lucy",
	"lucy_june":                "Lucy & June",
	"samantha_kirk":            "Samantha & Kirk",
	"tanner_chloe":             "Tanner & Chloe",
	"tyler_amy":                "Tyler & Amy",
	"decker":                   "The Deckers",
	"deinum":                   "The Deinums",
	"gantenbein":               "The Gantenbeins",
	"haas":                     "The Haases",
	"hopeman":                  "The Hopemans",
	"krafft":                   "The Kraffts",
	"mccoys":                   "The McCoys",
	"nelson":                   "The Nelsons",
	"olson":                    "The Olsons",
	"page":                     "The Pages",
	"seins":                    "The Seins",
	"shook":                    "The Shooks",
	"tompkins":                 "The Tompkinses",
	"tso":                      "The Tsos",
	"woodbury":                 "The Woodburys",
	"foothills_baptist_church": "Foothills Baptist Church",
}

var imageExts = map[string]bool{".jpeg": true, ".jpg": true, ".png": true, ".webp": true, ".gif": true}

type client struct {
	base  string
	token string
	http  *http.Client
}

type deck struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type card struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Media []json.RawMessage `json:"media"`
}

func main() {
	apiURL := flag.String("api", "", "base URL of the deployed API")
	dir := flag.String("dir", "assets", "directory of card images")
	dryRun := flag.Bool("dry-run", false, "print the cards that would be added and exit")
	flag.Parse()

	if err := run(context.Background(), *apiURL, *dir, *dryRun); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, apiURL, dir string, dryRun bool) error {
	files, err := cardFiles(dir)
	if err != nil {
		return err
	}
	if dryRun {
		for _, f := range files {
			fmt.Printf("%-28s -> %s\n", filepath.Base(f.path), f.name)
		}
		return nil
	}

	in := bufio.NewReader(os.Stdin)
	if apiURL == "" {
		apiURL, err = prompt(in, "API URL: ")
		if err != nil {
			return err
		}
	}
	refreshToken, err := prompt(in, "Refresh token (localStorage \"prayer-deck.tokens\" in the signed-in web app): ")
	if err != nil {
		return err
	}

	c := &client{base: strings.TrimRight(apiURL, "/"), http: &http.Client{Timeout: 2 * time.Minute}}
	if err := c.refresh(ctx, refreshToken); err != nil {
		return err
	}

	var decks struct {
		Decks []deck `json:"decks"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/decks", nil, "", &decks); err != nil {
		return fmt.Errorf("failed to list decks: %w", err)
	}
	if len(decks.Decks) == 0 {
		return errors.New("account has no decks")
	}
	d := decks.Decks[0]
	slog.InfoContext(ctx, "seeding deck", "deck", d.Name, "deck_id", d.ID)

	var existing struct {
		Cards []card `json:"cards"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/decks/"+d.ID+"/cards", nil, "", &existing); err != nil {
		return fmt.Errorf("failed to list cards: %w", err)
	}
	byName := make(map[string]card, len(existing.Cards))
	for _, ec := range existing.Cards {
		byName[ec.Name] = ec
	}

	for _, f := range files {
		ec, found := byName[f.name]
		if found && len(ec.Media) > 0 {
			slog.InfoContext(ctx, "skipping existing card", "name", f.name)
			continue
		}
		if !found {
			body, err := json.Marshal(map[string]string{"name": f.name})
			if err != nil {
				return fmt.Errorf("failed to encode card %q: %w", f.name, err)
			}
			if err := c.do(ctx, http.MethodPost, "/v1/decks/"+d.ID+"/cards", bytes.NewReader(body), "application/json", &ec); err != nil {
				return fmt.Errorf("failed to create card %q: %w", f.name, err)
			}
		}
		if err := c.upload(ctx, ec.ID, f.path); err != nil {
			return fmt.Errorf("failed to upload artwork for %q: %w", f.name, err)
		}
		slog.InfoContext(ctx, "added card", "name", f.name)
	}
	return nil
}

type cardFile struct {
	name string
	path string
}

// cardFiles returns the images in dir that have a card name, sorted by name.
func cardFiles(dir string) ([]cardFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", dir, err)
	}
	var files []cardFile
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || !imageExts[ext] {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		name, ok := cardNames[stem]
		if !ok {
			slog.Info("skipping image with no card name", "file", e.Name())
			continue
		}
		files = append(files, cardFile{name: name, path: filepath.Join(dir, e.Name())})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

func prompt(in *bufio.Reader, label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return "", errors.New("input is required")
	}
	return value, nil
}

func (c *client) refresh(ctx context.Context, refreshToken string) error {
	body, err := json.Marshal(map[string]string{"refresh_token": refreshToken})
	if err != nil {
		return fmt.Errorf("failed to encode refresh request: %w", err)
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/tokens/refresh", bytes.NewReader(body), "application/json", &tokens); err != nil {
		return fmt.Errorf("failed to refresh tokens: %w", err)
	}
	c.token = tokens.AccessToken
	return nil
}

func (c *client) upload(ctx context.Context, cardID, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", filepath.Base(path))
	if err != nil {
		return fmt.Errorf("failed to build upload: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return fmt.Errorf("failed to read image: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to build upload: %w", err)
	}
	return c.do(ctx, http.MethodPost, "/v1/cards/"+url.PathEscape(cardID)+"/media", &buf, w.FormDataContentType(), nil)
}

func (c *client) do(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("%s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}
