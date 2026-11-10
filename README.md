# Prayer Deck

A personal prayer practice built around illustrated cards of the people you love.

Each card holds one person, a couple, or a family: a name, a small hand-drawn
portrait, and a few personal touches such as a hobby, a pet, or a favorite place.
You shuffle the deck, draw three cards, turn them over one at a time, and spend a
few quiet minutes with each person in prayer.

Prayer Deck began as a deck of handmade paper cards. Making new cards by hand took
too long, and the deck stopped growing. This app keeps what made the paper deck
good (the shuffle, the reveal, the face looking back at you) and makes adding
people easy.

## The experience

- **Draw.** The deck rests on a linen table. Shuffle it, deal three cards face
  down, and turn each one over when you're ready. Tap someone to hold their card
  up close with the details that bring them to mind. Your current draw stays on
  the table until you draw again, so you can step away and come back.
- **Work through the deck.** Each draw comes from cards you haven't drawn yet in
  this pass through the deck. Once everyone has had a turn, the deck reshuffles.
- **Manage cards.** Add, edit, and remove cards and their artwork. Cards without
  artwork show a handwritten name until a drawing is added.
- **Decks.** Keep more than one deck, such as family, friends, or church, and
  draw from whichever you choose.

The app is deliberately small. It has no streaks, scores, feeds, or tracking.
It exists to help you remember people, not to measure prayer.

## Concepts

| Concept      | Meaning                                                                                                                                                   |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Account**  | You. You sign in with Google. More sign-in methods can be added later, and each one attaches to the same account and collection.                             |
| **Deck**     | A named collection of cards. Every account starts with one.                                                                                               |
| **Card**     | A person, couple, or family, with a display name, personal details, and artwork.                                                                          |
| **Artwork**  | The illustration on a card's face. Each upload is kept in its original form and also prepared in lighter sizes for the card face and for lists.           |
| **Pass**     | One trip through a deck. Cards drawn in the current pass wait until the deck reshuffles.                                                                  |
| **Draw**     | The hand most recently dealt from a deck. It lasts until the next draw.                                                                                   |

## How it fits together

- **API**: a Go service that owns accounts, decks, cards, and draws. It stores
  data in PostgreSQL and artwork in S3-compatible object storage.
- **Client**: one web interface packaged with [Wails](https://v3.wails.io). It is
  served to browsers today and is meant to ship to iOS later without a separate
  Swift interface.

Every client talks to the same hosted API, so a person's collection is the same
on the web and, later, on the phone.

## Running it locally

The full stack runs in Docker: PostgreSQL, MinIO, the API, and the web client.

```bash
make up        # build and start everything
open http://localhost:3000
make down      # stop and remove local data
```

Locally you can sign in with any email address through the development sign-in.
To use Google sign-in, set `GOOGLE_CLIENT_ID` in `.env` (see `.env.example`).

See [AGENTS.md](AGENTS.md) for how the repository is organized and how to work in it.
