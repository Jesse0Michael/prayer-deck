package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jesse0michael/pkg/auth"
	"github.com/jesse0michael/pkg/auth/oidc"
	"github.com/jesse0michael/pkg/test"
	"github.com/jesse0michael/prayer-deck/internal/ent"
	"github.com/jesse0michael/prayer-deck/internal/ent/asset"
	entmedia "github.com/jesse0michael/prayer-deck/internal/ent/media"
	"github.com/jesse0michael/prayer-deck/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	testUserID  = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	testDeckID  = uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	testCardID  = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	testNoteID  = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	testMediaID = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	testDrawID  = uuid.MustParse("00000000-0000-0000-0000-0000000000f1")
	testTime    = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	errTest     = errors.New("test-error")
)

func testDeck() *store.Deck {
	return &store.Deck{
		Deck:      &ent.Deck{ID: testDeckID, Name: "test-deck", ShuffledAt: testTime, CreatedAt: testTime, UpdatedAt: testTime},
		CardCount: 3,
		Remaining: 2,
	}
}

const testDeckJSON = `{"id":"00000000-0000-0000-0000-0000000000d1","name":"test-deck","card_count":3,"remaining":2,"created_at":"2026-11-09T12:00:00Z","updated_at":"2026-11-09T12:00:00Z"}`

func testCard() *ent.Card {
	return &ent.Card{ID: testCardID, Name: "test-card", CreatedAt: testTime, UpdatedAt: testTime}
}

const testCardJSON = `{"id":"00000000-0000-0000-0000-0000000000c1","name":"test-card","media":[],"created_at":"2026-11-09T12:00:00Z","updated_at":"2026-11-09T12:00:00Z"}`

func testCardWithMedia() *ent.Card {
	c := testCard()
	c.Edges.Media = []*ent.Media{{ID: testMediaID, Kind: entmedia.KindImage, CreatedAt: testTime, Edges: ent.MediaEdges{Assets: []*ent.Asset{
		{Rendition: asset.RenditionOriginal, Key: "test-original", Width: 1024, Height: 1536},
		{Rendition: asset.RenditionDisplay, Key: "test-display", Width: 1024, Height: 1536},
		{Rendition: asset.RenditionThumbnail, Key: "test-thumbnail", Width: 320, Height: 480},
	}}}}
	return c
}

const testMediaJSON = `{"id":"00000000-0000-0000-0000-0000000000b1","kind":"image","display_url":"https://storage.test/test-display","thumbnail_url":"https://storage.test/test-thumbnail","original_url":"https://storage.test/test-original","width":1024,"height":1536,"created_at":"2026-11-09T12:00:00Z"}`

const testCardWithMediaJSON = `{"id":"00000000-0000-0000-0000-0000000000c1","name":"test-card","media":[` + testMediaJSON + `],"created_at":"2026-11-09T12:00:00Z","updated_at":"2026-11-09T12:00:00Z"}`

func expectURLs(storage *MockStorage) {
	storage.EXPECT().URL(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string) (string, error) {
		return "https://storage.test/" + key, nil
	}).AnyTimes()
}

type testServer struct {
	server  *Server
	store   *MockStore
	storage *MockStorage
	token   string
}

func newTestServer(t *testing.T, opts ...Option) testServer {
	t.Helper()
	ctrl := gomock.NewController(t)
	st := NewMockStore(ctrl)
	storage := NewMockStorage(ctrl)
	jwtAuth := auth.NewJWTAuth(auth.Config{SecretKey: []byte("test-secret"), AccessTokenTTL: time.Hour, RefreshTokenTTL: time.Hour}, jwt.SigningMethodHS256)
	token, _, err := jwtAuth.GenerateTokens(auth.WithSubject(testUserID.String()))
	require.NoError(t, err)
	return testServer{server: NewServer(st, storage, jwtAuth, opts...), store: st, storage: storage, token: token}
}

// serve sends a request through the full routing table, authenticated unless token is empty.
func (ts testServer) serve(method, path, token, contentType string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, body)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	ts.server.Routes().ServeHTTP(w, r)
	return w
}

func TestServer_Auth(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "missing token",
			expectedCode: http.StatusForbidden,
			expectedBody: `{"errors":[{"message":"forbidden"}]}`,
		},
		{
			name:         "invalid token",
			token:        "test-invalid-token",
			expectedCode: http.StatusForbidden,
			expectedBody: `{"errors":[{"message":"forbidden"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)

			w := ts.serve(http.MethodGet, "/v1/decks", tt.token, "", nil)

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func TestServer_CORS(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		origin          string
		expectedCode    int
		expectedHeaders http.Header
	}{
		{
			name:         "preflight from an allowed origin",
			method:       http.MethodOptions,
			origin:       "http://localhost:3000",
			expectedCode: http.StatusNoContent,
			expectedHeaders: http.Header{
				"Access-Control-Allow-Origin":  {"http://localhost:3000"},
				"Access-Control-Allow-Methods": {"GET, POST, PUT, PATCH, DELETE"},
				"Access-Control-Allow-Headers": {"Authorization, Content-Type"},
				"Access-Control-Max-Age":       {"600"},
				"Vary":                         {"Origin"},
			},
		},
		{
			name:            "preflight from another origin",
			method:          http.MethodOptions,
			origin:          "http://test-origin.example",
			expectedCode:    http.StatusMethodNotAllowed,
			expectedHeaders: http.Header{"Allow": {"GET, HEAD"}, "Content-Type": {"text/plain; charset=utf-8"}, "X-Content-Type-Options": {"nosniff"}},
		},
		{
			name:         "request from an allowed origin",
			method:       http.MethodGet,
			origin:       "http://localhost:3000",
			expectedCode: http.StatusOK,
			expectedHeaders: http.Header{
				"Access-Control-Allow-Origin": {"http://localhost:3000"},
				"Content-Type":                {"application/json"},
				"Vary":                        {"Origin"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, WithAllowedOrigins("http://localhost:3000"))
			r := httptest.NewRequest(tt.method, "/v1/tokens", nil)
			r.Header.Set("Origin", tt.origin)
			r.Header.Set("Access-Control-Request-Method", http.MethodGet)
			w := httptest.NewRecorder()

			ts.server.Routes().ServeHTTP(w, r)

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedHeaders, w.Header())
		})
	}
}

func TestServer_listLoginProviders(t *testing.T) {
	tests := []struct {
		name         string
		opts         []Option
		expectedBody string
	}{
		{
			name:         "no providers",
			expectedBody: `{"providers":[]}`,
		},
		{
			name: "google and dev",
			opts: []Option{
				WithLoginProvider(LoginProvider{Name: "google", ClientID: "test-client-id", Verifier: DevVerifier{}}),
				WithLoginProvider(LoginProvider{Name: "dev", Verifier: DevVerifier{}}),
			},
			expectedBody: `{"providers":[{"name":"google","client_id":"test-client-id"},{"name":"dev"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, tt.opts...)

			w := ts.serve(http.MethodGet, "/v1/tokens", "", "", nil)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func TestServer_resolveIdentity(t *testing.T) {
	tests := []struct {
		name         string
		claims       *oidc.Claims
		mockSetup    func(st *MockStore)
		expectedCode int
		expectedBody string
	}{
		{
			name:   "issues tokens for the resolved account",
			claims: &oidc.Claims{Provider: "google", Subject: "test-subject", Email: "test@example.com", EmailVerified: true, Name: "test-name", Picture: "test-picture"},
			mockSetup: func(st *MockStore) {
				st.EXPECT().ResolveIdentity(gomock.Any(), store.Identity{Provider: "google", Subject: "test-subject", Email: "test@example.com", EmailVerified: true, Name: "test-name", PictureURL: "test-picture"}).
					Return(&ent.User{ID: testUserID}, nil)
			},
			expectedCode: http.StatusOK,
		},
		{
			name:         "provider without email",
			claims:       &oidc.Claims{Provider: "google", Subject: "test-subject"},
			mockSetup:    func(*MockStore) {},
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{"errors":[{"message":"provider did not share an email"}]}`,
		},
		{
			name:   "email claimed by another account",
			claims: &oidc.Claims{Provider: "google", Subject: "test-subject", Email: "test@example.com"},
			mockSetup: func(st *MockStore) {
				st.EXPECT().ResolveIdentity(gomock.Any(), gomock.Any()).Return(nil, store.ErrConflict)
			},
			expectedCode: http.StatusConflict,
			expectedBody: `{"errors":[{"message":"conflict","details":"conflict"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, WithLoginProvider(LoginProvider{Name: "test", Verifier: staticVerifier{claims: tt.claims}}))
			tt.mockSetup(ts.store)

			w := ts.serve(http.MethodPost, "/v1/tokens/test", "", "application/json", strings.NewReader(`{"id_token":"test-token"}`))

			require.Equal(t, tt.expectedCode, w.Code)
			if tt.expectedCode == http.StatusOK {
				require.Contains(t, w.Body.String(), `"subject":"`+testUserID.String()+`"`)
				return
			}
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

type staticVerifier struct {
	claims *oidc.Claims
}

func (v staticVerifier) Verify(context.Context, string) (*oidc.Claims, error) {
	return v.claims, nil
}

func TestDevVerifier_Verify(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		expected  *oidc.Claims
		wantError bool
	}{
		{
			name:     "email address",
			token:    " Test-User@Example.com ",
			expected: &oidc.Claims{Provider: "dev", Subject: "test-user@example.com", Email: "test-user@example.com", EmailVerified: true, Name: "test-user"},
		},
		{
			name:      "not an email address",
			token:     "test-user",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := DevVerifier{}.Verify(t.Context(), tt.token)

			require.Equal(t, tt.expected, claims)
			require.Equal(t, tt.wantError, err != nil)
		})
	}
}

func TestServer_getMe(t *testing.T) {
	tests := []struct {
		name         string
		mockSetup    func(st *MockStore)
		expectedCode int
		expectedBody string
	}{
		{
			name: "returns the account",
			mockSetup: func(st *MockStore) {
				st.EXPECT().User(gomock.Any(), testUserID).Return(&ent.User{ID: testUserID, Email: "test@example.com", Name: "test-name"}, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"id":"00000000-0000-0000-0000-0000000000a1","email":"test@example.com","name":"test-name"}`,
		},
		{
			name: "account removed",
			mockSetup: func(st *MockStore) {
				st.EXPECT().User(gomock.Any(), testUserID).Return(nil, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			tt.mockSetup(ts.store)

			w := ts.serve(http.MethodGet, "/v1/me", ts.token, "", nil)

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func TestServer_decks(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		mockSetup    func(st *MockStore, storage *MockStorage)
		expectedCode int
		expectedBody string
	}{
		{
			name:   "list decks",
			method: http.MethodGet,
			path:   "/v1/decks",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Decks(gomock.Any(), testUserID).Return([]*store.Deck{testDeck()}, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"decks":[` + testDeckJSON + `]}`,
		},
		{
			name:   "list decks fails",
			method: http.MethodGet,
			path:   "/v1/decks",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Decks(gomock.Any(), testUserID).Return(nil, errTest)
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"errors":[{"message":"Internal Server Error"}]}`,
		},
		{
			name:   "create deck",
			method: http.MethodPost,
			path:   "/v1/decks",
			body:   `{"name":"  test-deck  "}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().CreateDeck(gomock.Any(), testUserID, "test-deck").Return(testDeck(), nil)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testDeckJSON,
		},
		{
			name:         "create deck without a name",
			method:       http.MethodPost,
			path:         "/v1/decks",
			body:         `{"name":" "}`,
			mockSetup:    func(*MockStore, *MockStorage) {},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"errors":[{"message":"name is required"}]}`,
		},
		{
			name:   "rename deck",
			method: http.MethodPatch,
			path:   "/v1/decks/" + testDeckID.String(),
			body:   `{"name":"test-deck"}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().RenameDeck(gomock.Any(), testUserID, testDeckID, "test-deck").Return(testDeck(), nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: testDeckJSON,
		},
		{
			name:         "rename deck with an invalid id",
			method:       http.MethodPatch,
			path:         "/v1/decks/test-invalid-id",
			body:         `{"name":"test-deck"}`,
			mockSetup:    func(*MockStore, *MockStorage) {},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
		{
			name:   "delete deck and its artwork",
			method: http.MethodDelete,
			path:   "/v1/decks/" + testDeckID.String(),
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().DeleteDeck(gomock.Any(), testUserID, testDeckID).Return([]string{"test-key-1", "test-key-2"}, nil)
				storage.EXPECT().Delete(gomock.Any(), "test-key-1", "test-key-2").Return(nil)
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "delete deck succeeds when artwork cleanup fails",
			method: http.MethodDelete,
			path:   "/v1/decks/" + testDeckID.String(),
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().DeleteDeck(gomock.Any(), testUserID, testDeckID).Return([]string{"test-key"}, nil)
				storage.EXPECT().Delete(gomock.Any(), "test-key").Return(errTest)
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "delete the last deck",
			method: http.MethodDelete,
			path:   "/v1/decks/" + testDeckID.String(),
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().DeleteDeck(gomock.Any(), testUserID, testDeckID).Return(nil, errors.Join(store.ErrConflict, errors.New("an account keeps at least one deck")))
			},
			expectedCode: http.StatusConflict,
			expectedBody: `{"errors":[{"message":"conflict","details":"conflict\nan account keeps at least one deck"}]}`,
		},
		{
			name:   "shuffle deck",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/shuffle",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Shuffle(gomock.Any(), testUserID, testDeckID).Return(testDeck(), nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: testDeckJSON,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			tt.mockSetup(ts.store, ts.storage)

			w := ts.serve(tt.method, tt.path, ts.token, "application/json", strings.NewReader(tt.body))

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func TestServer_cards(t *testing.T) {
	name := "test-card"
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		mockSetup    func(st *MockStore, storage *MockStorage)
		expectedCode int
		expectedBody string
	}{
		{
			name:   "list cards",
			method: http.MethodGet,
			path:   "/v1/decks/" + testDeckID.String() + "/cards",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().Cards(gomock.Any(), testUserID, testDeckID).Return([]*ent.Card{testCardWithMedia(), testCard()}, nil)
				expectURLs(storage)
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"cards":[` + testCardWithMediaJSON + `,` + testCardJSON + `]}`,
		},
		{
			name:   "list cards of a missing deck",
			method: http.MethodGet,
			path:   "/v1/decks/" + testDeckID.String() + "/cards",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Cards(gomock.Any(), testUserID, testDeckID).Return(nil, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
		{
			name:   "list cards fails to sign media",
			method: http.MethodGet,
			path:   "/v1/decks/" + testDeckID.String() + "/cards",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().Cards(gomock.Any(), testUserID, testDeckID).Return([]*ent.Card{testCardWithMedia()}, nil)
				storage.EXPECT().URL(gomock.Any(), "test-original").Return("", errTest)
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"errors":[{"message":"Internal Server Error"}]}`,
		},
		{
			name:   "create card",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/cards",
			body:   `{"name":" test-card "}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().CreateCard(gomock.Any(), testUserID, testDeckID, store.CardInput{Name: &name}).Return(testCard(), nil)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testCardJSON,
		},
		{
			name:   "create card without a name",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/cards",
			body:   `{}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().CreateCard(gomock.Any(), testUserID, testDeckID, store.CardInput{}).Return(testCard(), nil)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testCardJSON,
		},
		{
			name:   "get card",
			method: http.MethodGet,
			path:   "/v1/cards/" + testCardID.String(),
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().Card(gomock.Any(), testUserID, testCardID).Return(testCardWithMedia(), nil)
				expectURLs(storage)
			},
			expectedCode: http.StatusOK,
			expectedBody: testCardWithMediaJSON,
		},
		{
			name:   "update card name",
			method: http.MethodPatch,
			path:   "/v1/cards/" + testCardID.String(),
			body:   `{"name":"test-card"}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().UpdateCard(gomock.Any(), testUserID, testCardID, store.CardInput{Name: &name}).Return(testCard(), nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: testCardJSON,
		},
		{
			name:   "update card with a blank name",
			method: http.MethodPatch,
			path:   "/v1/cards/" + testCardID.String(),
			body:   `{"name":"  "}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				blank := ""
				st.EXPECT().UpdateCard(gomock.Any(), testUserID, testCardID, store.CardInput{Name: &blank}).Return(testCard(), nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: testCardJSON,
		},
		{
			name:   "delete card and its media",
			method: http.MethodDelete,
			path:   "/v1/cards/" + testCardID.String(),
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().DeleteCard(gomock.Any(), testUserID, testCardID).Return([]string{"test-key"}, nil)
				storage.EXPECT().Delete(gomock.Any(), "test-key").Return(nil)
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "delete another user's card",
			method: http.MethodDelete,
			path:   "/v1/cards/" + testCardID.String(),
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().DeleteCard(gomock.Any(), testUserID, testCardID).Return(nil, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			tt.mockSetup(ts.store, ts.storage)

			w := ts.serve(tt.method, tt.path, ts.token, "application/json", strings.NewReader(tt.body))

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func testNote() *ent.Note {
	return &ent.Note{ID: testNoteID, Text: "test-note", CreatedAt: testTime, UpdatedAt: testTime}
}

const testNoteJSON = `{"id":"00000000-0000-0000-0000-0000000000a2","text":"test-note","created_at":"2026-11-09T12:00:00Z","updated_at":"2026-11-09T12:00:00Z"}`

func TestServer_notes(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		mockSetup    func(st *MockStore)
		expectedCode int
		expectedBody string
	}{
		{
			name:   "list notes with the default page size",
			method: http.MethodGet,
			path:   "/v1/cards/" + testCardID.String() + "/notes",
			mockSetup: func(st *MockStore) {
				st.EXPECT().Notes(gomock.Any(), testUserID, testCardID, 6, 0).Return([]*ent.Note{testNote()}, 7, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"notes":[` + testNoteJSON + `],"total":7}`,
		},
		{
			name:   "list notes with a page",
			method: http.MethodGet,
			path:   "/v1/cards/" + testCardID.String() + "/notes?limit=2&offset=4",
			mockSetup: func(st *MockStore) {
				st.EXPECT().Notes(gomock.Any(), testUserID, testCardID, 2, 4).Return(nil, 4, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: `{"notes":[],"total":4}`,
		},
		{
			name:         "list notes with an invalid limit",
			method:       http.MethodGet,
			path:         "/v1/cards/" + testCardID.String() + "/notes?limit=0",
			mockSetup:    func(*MockStore) {},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"errors":[{"message":"invalid limit","details":"must be between 1 and 50"}]}`,
		},
		{
			name:   "list notes of another user's card",
			method: http.MethodGet,
			path:   "/v1/cards/" + testCardID.String() + "/notes",
			mockSetup: func(st *MockStore) {
				st.EXPECT().Notes(gomock.Any(), testUserID, testCardID, 6, 0).Return(nil, 0, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
		{
			name:   "create note",
			method: http.MethodPost,
			path:   "/v1/cards/" + testCardID.String() + "/notes",
			body:   `{"text":" test-note "}`,
			mockSetup: func(st *MockStore) {
				st.EXPECT().CreateNote(gomock.Any(), testUserID, testCardID, "test-note").Return(testNote(), nil)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testNoteJSON,
		},
		{
			name:         "create note without text",
			method:       http.MethodPost,
			path:         "/v1/cards/" + testCardID.String() + "/notes",
			body:         `{"text":"  "}`,
			mockSetup:    func(*MockStore) {},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"errors":[{"message":"text must not be empty"}]}`,
		},
		{
			name:         "create note that is too long",
			method:       http.MethodPost,
			path:         "/v1/cards/" + testCardID.String() + "/notes",
			body:         `{"text":"` + strings.Repeat("a", 2001) + `"}`,
			mockSetup:    func(*MockStore) {},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"errors":[{"message":"text is too long","details":"limit is 2000 characters"}]}`,
		},
		{
			name:   "update note",
			method: http.MethodPatch,
			path:   "/v1/notes/" + testNoteID.String(),
			body:   `{"text":"test-note"}`,
			mockSetup: func(st *MockStore) {
				st.EXPECT().UpdateNote(gomock.Any(), testUserID, testNoteID, "test-note").Return(testNote(), nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: testNoteJSON,
		},
		{
			name:   "update another user's note",
			method: http.MethodPatch,
			path:   "/v1/notes/" + testNoteID.String(),
			body:   `{"text":"test-note"}`,
			mockSetup: func(st *MockStore) {
				st.EXPECT().UpdateNote(gomock.Any(), testUserID, testNoteID, "test-note").Return(nil, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
		{
			name:   "delete note",
			method: http.MethodDelete,
			path:   "/v1/notes/" + testNoteID.String(),
			mockSetup: func(st *MockStore) {
				st.EXPECT().DeleteNote(gomock.Any(), testUserID, testNoteID).Return(nil)
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name:   "delete another user's note",
			method: http.MethodDelete,
			path:   "/v1/notes/" + testNoteID.String(),
			mockSetup: func(st *MockStore) {
				st.EXPECT().DeleteNote(gomock.Any(), testUserID, testNoteID).Return(store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			tt.mockSetup(ts.store)

			w := ts.serve(tt.method, tt.path, ts.token, "application/json", strings.NewReader(tt.body))

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func TestServer_addMedia(t *testing.T) {
	keyPrefix := "cards/" + testCardID.String() + "/"
	mediaPrefix := keyPrefix + "52fdfc07-2182-454f-963f-5f0f9a621d72/"
	originalKey := mediaPrefix + "original.png"
	displayKey := mediaPrefix + "display.jpg"
	thumbnailKey := mediaPrefix + "thumbnail.jpg"
	tests := []struct {
		name         string
		file         string
		mockSetup    func(st *MockStore, storage *MockStorage)
		expectedCode int
		expectedBody string
	}{
		{
			name: "stores every rendition as new media",
			file: "testdata/artwork.png",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().Card(gomock.Any(), testUserID, testCardID).Return(testCard(), nil)
				storage.EXPECT().Put(gomock.Any(), originalKey, gomock.Any(), int64(148), "image/png").Return(nil)
				storage.EXPECT().Put(gomock.Any(), displayKey, gomock.Any(), gomock.Any(), "image/jpeg").Return(nil)
				storage.EXPECT().Put(gomock.Any(), thumbnailKey, gomock.Any(), gomock.Any(), "image/jpeg").Return(nil)
				st.EXPECT().AddMedia(gomock.Any(), testUserID, testCardID, gomock.Any()).
					DoAndReturn(func(_ context.Context, _, _ uuid.UUID, in store.MediaInput) (*ent.Card, error) {
						require.Equal(t, uuid.MustParse("52fdfc07-2182-454f-963f-5f0f9a621d72"), in.ID)
						require.Equal(t, entmedia.KindImage, in.Kind)
						keys := make([]string, len(in.Assets))
						for i, a := range in.Assets {
							keys[i] = a.Key
						}
						require.Equal(t, []string{originalKey, displayKey, thumbnailKey}, keys)
						return testCardWithMedia(), nil
					})
				expectURLs(storage)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testCardWithMediaJSON,
		},
		{
			name: "removes uploaded renditions when saving fails",
			file: "testdata/artwork.png",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().Card(gomock.Any(), testUserID, testCardID).Return(testCard(), nil)
				storage.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(3)
				st.EXPECT().AddMedia(gomock.Any(), testUserID, testCardID, gomock.Any()).Return(nil, errTest)
				storage.EXPECT().Delete(gomock.Any(), originalKey, displayKey, thumbnailKey).Return(nil)
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"errors":[{"message":"Internal Server Error"}]}`,
		},
		{
			name: "removes earlier renditions when an upload fails",
			file: "testdata/artwork.png",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().Card(gomock.Any(), testUserID, testCardID).Return(testCard(), nil)
				storage.EXPECT().Put(gomock.Any(), originalKey, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				storage.EXPECT().Put(gomock.Any(), displayKey, gomock.Any(), gomock.Any(), gomock.Any()).Return(errTest)
				storage.EXPECT().Delete(gomock.Any(), originalKey).Return(nil)
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"errors":[{"message":"Internal Server Error"}]}`,
		},
		{
			name: "rejects files that are not images",
			file: "testdata/not-image.txt",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Card(gomock.Any(), testUserID, testCardID).Return(testCard(), nil)
			},
			expectedCode: http.StatusUnsupportedMediaType,
			expectedBody: `{"errors":[{"message":"unsupported image","details":"upload a PNG, JPEG, WebP, or GIF"}]}`,
		},
		{
			name: "another user's card",
			file: "testdata/artwork.png",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Card(gomock.Any(), testUserID, testCardID).Return(nil, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			tt.mockSetup(ts.store, ts.storage)
			uuid.SetRand(rand.New(rand.NewSource(1)))
			defer uuid.SetRand(nil)
			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			part, err := mw.CreateFormFile("image", "test-image")
			require.NoError(t, err)
			_, err = part.Write(test.LoadFile(t, tt.file))
			require.NoError(t, err)
			require.NoError(t, mw.Close())

			w := ts.serve(http.MethodPost, "/v1/cards/"+testCardID.String()+"/media", ts.token, mw.FormDataContentType(), &body)

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func TestServer_deleteMedia(t *testing.T) {
	tests := []struct {
		name         string
		mockSetup    func(st *MockStore, storage *MockStorage)
		expectedCode int
		expectedBody string
	}{
		{
			name: "deletes the media and its objects",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().DeleteMedia(gomock.Any(), testUserID, testCardID, testMediaID).Return([]string{"test-display", "test-thumbnail"}, nil)
				storage.EXPECT().Delete(gomock.Any(), "test-display", "test-thumbnail").Return(nil)
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name: "succeeds when object cleanup fails",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().DeleteMedia(gomock.Any(), testUserID, testCardID, testMediaID).Return([]string{"test-display"}, nil)
				storage.EXPECT().Delete(gomock.Any(), "test-display").Return(errTest)
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name: "another user's media",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().DeleteMedia(gomock.Any(), testUserID, testCardID, testMediaID).Return(nil, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			tt.mockSetup(ts.store, ts.storage)

			w := ts.serve(http.MethodDelete, "/v1/cards/"+testCardID.String()+"/media/"+testMediaID.String(), ts.token, "", nil)

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

func TestServer_draws(t *testing.T) {
	testDraw := &store.Draw{
		Draw:      &ent.Draw{ID: testDrawID, CardIds: []uuid.UUID{testCardID}, CreatedAt: testTime},
		Cards:     []*ent.Card{testCard()},
		Remaining: 4,
	}
	testDrawJSON := `{"id":"00000000-0000-0000-0000-0000000000f1","cards":[` + strings.TrimSuffix(testCardJSON, "}") + `,"face":null}],"discard":[],"remaining":4,"created_at":"2026-11-09T12:00:00Z"}`
	testDrawWithFace := &store.Draw{
		Draw:      &ent.Draw{ID: testDrawID, CardIds: []uuid.UUID{testCardID}, Faces: map[uuid.UUID]uuid.UUID{testCardID: testMediaID}, CreatedAt: testTime},
		Cards:     []*ent.Card{testCardWithMedia()},
		Discard:   []*ent.Card{testCardWithMedia()},
		Remaining: 4,
	}
	testDrawWithFaceJSON := `{"id":"00000000-0000-0000-0000-0000000000f1","cards":[` + strings.TrimSuffix(testCardWithMediaJSON, "}") + `,"face":` + testMediaJSON + `}],"discard":[` + testCardWithMediaJSON + `],"remaining":4,"created_at":"2026-11-09T12:00:00Z"}`
	testDrawWithDeletedFace := &store.Draw{
		Draw:      &ent.Draw{ID: testDrawID, CardIds: []uuid.UUID{testCardID}, Faces: map[uuid.UUID]uuid.UUID{testCardID: uuid.MustParse("00000000-0000-0000-0000-0000000000b2")}, CreatedAt: testTime},
		Cards:     []*ent.Card{testCardWithMedia()},
		Remaining: 4,
	}
	testDrawWithDeletedFaceJSON := `{"id":"00000000-0000-0000-0000-0000000000f1","cards":[` + strings.TrimSuffix(testCardWithMediaJSON, "}") + `,"face":null}],"discard":[],"remaining":4,"created_at":"2026-11-09T12:00:00Z"}`
	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		mockSetup    func(st *MockStore, storage *MockStorage)
		expectedCode int
		expectedBody string
	}{
		{
			name:   "draw the default three",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/draws",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Draw(gomock.Any(), testUserID, testDeckID, 3, false).Return(testDraw, nil)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testDrawJSON,
		},
		{
			name:   "draw a chosen count",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/draws",
			body:   `{"count":1}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Draw(gomock.Any(), testUserID, testDeckID, 1, false).Return(testDraw, nil)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testDrawJSON,
		},
		{
			name:   "shuffle and draw",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/draws",
			body:   `{"count":3,"shuffle":true}`,
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Draw(gomock.Any(), testUserID, testDeckID, 3, true).Return(testDraw, nil)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testDrawJSON,
		},
		{
			name:   "draw shows each card's chosen face",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/draws",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().Draw(gomock.Any(), testUserID, testDeckID, 3, false).Return(testDrawWithFace, nil)
				expectURLs(storage)
			},
			expectedCode: http.StatusCreated,
			expectedBody: testDrawWithFaceJSON,
		},
		{
			name:   "current draw whose face was deleted",
			method: http.MethodGet,
			path:   "/v1/decks/" + testDeckID.String() + "/draws/current",
			mockSetup: func(st *MockStore, storage *MockStorage) {
				st.EXPECT().CurrentDraw(gomock.Any(), testUserID, testDeckID).Return(testDrawWithDeletedFace, nil)
				expectURLs(storage)
			},
			expectedCode: http.StatusOK,
			expectedBody: testDrawWithDeletedFaceJSON,
		},
		{
			name:         "draw too many",
			method:       http.MethodPost,
			path:         "/v1/decks/" + testDeckID.String() + "/draws",
			body:         `{"count":11}`,
			mockSetup:    func(*MockStore, *MockStorage) {},
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"errors":[{"message":"count must be between 1 and 10"}]}`,
		},
		{
			name:   "draw from an empty deck",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/draws",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Draw(gomock.Any(), testUserID, testDeckID, 3, false).Return(nil, store.ErrEmptyDeck)
			},
			expectedCode: http.StatusUnprocessableEntity,
			expectedBody: `{"errors":[{"message":"deck has no cards"}]}`,
		},
		{
			name:   "draw after the pass is finished",
			method: http.MethodPost,
			path:   "/v1/decks/" + testDeckID.String() + "/draws",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().Draw(gomock.Any(), testUserID, testDeckID, 3, false).Return(nil, store.ErrPassFinished)
			},
			expectedCode: http.StatusConflict,
			expectedBody: `{"errors":[{"message":"every card has been drawn; shuffle to start again"}]}`,
		},
		{
			name:   "current draw",
			method: http.MethodGet,
			path:   "/v1/decks/" + testDeckID.String() + "/draws/current",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().CurrentDraw(gomock.Any(), testUserID, testDeckID).Return(testDraw, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: testDrawJSON,
		},
		{
			name:   "no current draw",
			method: http.MethodGet,
			path:   "/v1/decks/" + testDeckID.String() + "/draws/current",
			mockSetup: func(st *MockStore, _ *MockStorage) {
				st.EXPECT().CurrentDraw(gomock.Any(), testUserID, testDeckID).Return(nil, store.ErrNotFound)
			},
			expectedCode: http.StatusNotFound,
			expectedBody: `{"errors":[{"message":"not found"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t)
			tt.mockSetup(ts.store, ts.storage)

			w := ts.serve(tt.method, tt.path, ts.token, "application/json", strings.NewReader(tt.body))

			require.Equal(t, tt.expectedCode, w.Code)
			require.Equal(t, tt.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}
