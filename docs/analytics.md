# Web analytics

Terraform creates Prayer Deck's own GA4 property and web stream under the
Analytics account selected by `GA_ACCOUNT_ID`. The property appears after the
main `deploy/` configuration is applied. The published web app
reports visits to four screens and successful actions. These events can answer
questions such as how often visitors try a deal in the demo, how many sign in,
and whether people use the card editor in their own decks.

| Event | When it fires | App-supplied fields |
| --- | --- | --- |
| `page_view` | Entering the landing page, sign-in, Draw, or Cards. The embedded demo stays on the landing page. | A fixed title and virtual path of `/`, `/sign-in`, `/draw`, or `/cards` |
| `login` | Google sign-in succeeds. Restoring an existing session does not fire this event. | `method=google` |
| `hand_dealt` | A new hand is dealt, including a short hand near the end of a pass. | `deck_mode`, `draw_type` (`shuffle` or `next`), and `hand_size` (cards actually dealt) |
| `card_created` | A card is created, including when adding its first note creates it in the editor. | `deck_mode` |
| `card_updated` | A save succeeds in the editor for an existing card. | `deck_mode` |
| `card_deleted` | A confirmed deletion succeeds. | `deck_mode` |

In a GA4 Exploration, filter to `hand_dealt` and compare event counts by deck
mode (`demo` or `personal`) and draw type. For the path from a visit to sign-in,
build a funnel from `page_view` to the `login` key event. Returning users can
sign in again, so this measures successful logins rather than account creation.

GA4 uses its usual browser cookies and collects standard visit and device
information. The app supplies fixed page locations and omits the referring URL,
which limits campaign and referral attribution.
Enhanced measurement, Google Signals, and ad personalization are disabled.
App events exclude names, notes, artwork, email addresses, and account, card,
or deck IDs. Revealing a card and spending time with it generate no custom events.
Event-level retention is two months.

Terraform switches off enhanced measurement through the Analytics Admin API
when creating the stream. If someone later changes that setting in the GA4
console, reapply it from `deploy/` with
`terraform apply -replace=terraform_data.enhanced_measurement`.

## Verify the rollout

1. Complete [Enable analytics and publish](deploy.md#enable-analytics-and-publish).
   In Google Analytics, open the account/property selector, choose the account,
   and select **Prayer Deck**. To confirm its ID, run
   `terraform -chdir=deploy output -raw analytics_property_id` from the repository
   root. Publish the web
   client. The allowed hostname and measurement ID come from the applied
   Terraform configuration.
2. Open the production site in a browser that allows Analytics. Deal a hand in
   the demo, then open *Reports → Realtime* in the Prayer Deck property and
   look for `page_view` and `hand_dealt`. For the sign-in path, complete Google
   sign-in and check for `login`. Custom definitions can take longer to appear
   in standard reporting.
3. For local verification, run the frontend with the debug queue enabled:

   ```sh
   cd app/frontend
   VITE_ANALYTICS_DEBUG=true VITE_GA_MEASUREMENT_ID=G-TEST000001 npm run dev
   ```

   Open the demo, deal a hand, and inspect `window.dataLayer` in the browser
   console. Debug mode keeps commands in the browser without loading the Google
   tag. Ordinary local builds and preview hostnames leave analytics off.
