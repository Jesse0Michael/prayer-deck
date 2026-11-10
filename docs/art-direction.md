# Art direction (proposal)

The Rhett and Matt examples in `docs/art/` share a palette and medium but differ
in realism, line weight, framing, proportions, and where the name sits. A broad
text prompt alone won't produce a consistent deck. This is a starting proposal
for repeatable rules. Nothing here is decided until an approved reference card
exists.

## What the app assumes

- **Card ratio 2:3**, portrait, 1024×1536 or larger.
- **Full-bleed artwork.** The app draws the card edge, corner radius, and shadow,
  so the artwork shouldn't include a border.
- **Warm off-white ground** close to `#FBF6EC`, which the app uses as card stock
  and lays under transparent images.
- **The name is part of the artwork** today. Cards without artwork show a
  handwritten name in the app instead.

## Proposed composition rules

1. **Name** handwritten in black pen, centered in the top 20% of the card.
2. **Subject** from the waist or chest up, head in the upper-middle third. Use full
   body only when a defining object needs it (a golf club, a guitar).
3. **Line:** one consistent black pen weight. Loose but confident, with no
   cross-hatched shading.
4. **Color:** colored-pencil fill on clothing and one or two defining objects,
   with skin and hair lightly tinted. Leave most of the card blank.
5. **One personal object** from the card's details (a pet, a mug, a tool) beside
   the subject, not as a background scene.
6. **No background** beyond the paper.
7. Couples and families stand shoulder to shoulder in one row at the same
   framing, so a family card reads at the same scale as a single person.

## Getting to consistency

- Approve **one** reference card, then generate every card with that card as a
  style reference (image prompt) alongside the likeness photo.
- Keep a fixed prompt template in this file once it's proven, varying only the
  subject description and the personal object.
- If drift persists, a small LoRA trained on 15–30 approved cards is the
  long-term fix (a future feature, not required now).
