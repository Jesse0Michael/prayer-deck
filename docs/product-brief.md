# Prayer Deck Project Instructions

Help me develop Prayer Deck: a personal Christian prayer app built around illustrated cards representing friends, relatives, couples, and families. This is a web-first project with a future iOS app in mind. Work out implementation details with me as needed.

## Purpose and origin

I previously made physical cards by hand, with someone's name and a small illustration. I shuffled the deck, drew three cards, and spent time thinking about those people and praying for them. The cards helped bring specific people to heart and mind without making prayer feel mechanical. Creating new cards took too much effort, and I stopped maintaining the deck.

I now want to explore a digital version that I actually enjoy using. The visual and interactive experience is a major reason to build it. Do not redirect the project toward manually making and printing physical cards.

## Core experience

Keep the product centered on two activities:

- Draw cards: shuffle a deck, deal cards, reveal people, and spend time with them in prayer.
- Manage cards: add, view, edit, and remove cards and their artwork.

Three cards is the starting draw size, based on the original practice. A card can represent one person, a couple, or a family. Include a display name and recognizable artwork. Allow room for personal details such as a hobby, pet, favorite object, or meaningful place.

The app should feel personal, warm, and tactile. The focus is remembering people and praying for them. Avoid turning the experience into a productivity dashboard or measuring religious performance. Streaks, scores, competitive features, social feeds, and extensive tracking are outside the initial scope.

## Visual quality is part of the first version

I want fluid, seamless, satisfying card interactions: shuffling, dealing, flipping, stacking, lifting, and a sense of depth. A small feature set can still have ambitious visual polish.

Use perspective and motion thoughtfully so the cards feel somewhat three-dimensional. Support touch as well as a desktop pointer, with a phone-friendly layout. Prioritize responsive interaction and smooth animation on representative phones and computers; validate performance rather than promising that every device will be lag-free.

Card faces may use images and eventually short video loops. Card movement and animation within the artwork are separate capabilities. The interface must remain pleasing and usable with still artwork alone. Loading media should not interrupt a reveal or leave a blank face unexpectedly.

## Artwork direction

The original cards used simple black pen drawings, sparse color, handwritten names, and generous blank space. Early AI illustrations explored loose pen outlines and colored-pencil fills on warm off-white cards. Recognizable features and personality matter more than realism or elaborate detail.

These are visual references, not a finalized production style. Two generated examples, Rhett and Matt, varied substantially in realism, line thickness, framing, proportions, and layout despite similar prompts. A broad text description alone did not establish a consistent deck.

Help establish a coherent art direction using an approved visual reference and repeatable composition rules. Do not assume either example is the final chosen style.

Photos of people or families can be supplied as likeness references. Initial artwork can be generated manually outside the application and uploaded. Integrated AI generation is not required for the first version.

## Technical direction

Confirmed direction:

- Use Go for the hosted backend. I enjoy Go and want to keep using it.
- Use PostgreSQL for persistent application data.
- Store images and videos externally from the application bundle. The storage provider is undecided.
- Use Google OAuth for the initial account sign-in experience. Avoid building a custom password system.
- Launch as a website first, accessible from a phone and computer.
- Preserve a practical path toward an iOS App Store release while sharing as much code and interface work as possible.
- Desktop distribution is a low priority.

Ent is the preferred data-layer candidate. I have used it before and liked it, although it had quirks. It was recommended during exploration, but the exact stack has not been fully finalized. Do not replace it simply because another library is newer; explain a concrete project benefit if proposing a change.

Wails v3 is a serious candidate because of its Go integration and potential web/iOS code reuse. It is not yet an irrevocable framework decision. Its documentation describes a browser/server build and an iOS distribution path, while iOS remains experimental. Verify the current release's actual capabilities before committing the project to that path.

I want to avoid maintaining a separate Swift interface. Shared code is a goal, not a demand to pretend platform-specific work disappears. If another approach offers a materially more reliable web-to-iOS route while preserving Go on the backend, explain the tradeoff before changing direction.

The hosted backend and PostgreSQL should serve the same account and collection across clients. Reusing code in an iOS package does not itself synchronize data. Do not interpret shared Go code as a requirement to run the hosted database layer on the phone.

This is independent of my game platform. Do not inherit Unity, Godot, worlds/scenes, or a gRPC requirement from other game-development work.

## Accounts and release timing

Google sign-in is the initial plan. Sign in with Apple can be added when preparing for iOS distribution. I am not paying for Apple Developer enrollment yet; that does not block web development.

Before an App Store release, verify Apple's current distribution and login requirements. Plan for an equivalent privacy-preserving sign-in option alongside Google as required. Preserve account identity when adding providers so one person's cards do not become separate collections under different logins.

## Future possibilities

These are exciting directions, not first-version requirements:

- Generate artwork from reference photos within the app.
- Use a model with a trained LoRA or another appropriate method to make the art style more consistent.
- Maintain several artwork variants for a card.
- Animate the illustrated subjects or use short video faces.
- Send optional prayer reminders or push notifications.
- Release on iOS through the App Store.

Leave sensible room for these ideas without building their infrastructure in advance.

## Open choices

Resolve these when they become relevant rather than silently treating them as agreed requirements:

- Frontend framework and animation approach.
- Wails versus another web/iOS delivery strategy.
- Hosting and media storage providers; affordability matters, but no provider or zero-cost guarantee has been selected.
- Exact shuffle and repeat behavior, including whether a deck is exhausted before reshuffling.
- Whether multiple decks are needed initially.
- Final visual style, card framing, and name rendering.
- Offline behavior and how long a current draw should persist.

## How to work with me

Favor a focused, usable app with excellent card interactions. Do not expand the feature list merely because prayer apps commonly have journals, requests, devotionals, or community tools. Explain meaningful tradeoffs in plain language and distinguish my decisions from your recommendations.

Begin by absorbing this brief and proposing a compact first milestone that combines a polished draw experience with basic card management. Start from the decisions here; ask only about choices that materially affect the next step. Keep the initial experience useful with uploaded artwork, and validate the intended browser-to-iOS path early if selecting Wails.
