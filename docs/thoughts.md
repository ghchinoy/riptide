# Computer Use ideas

## local - headless

- The local use of a browser is the core application
- the visual record of the screenshots is shown at the end of the interaction

## local - browser visible

- a web application (Lit frontend) with the core Go backend where the Frontend shows the Chrome browser orchestrated by the chromedp; the user initiating the session can watch the interaction

## remote - headless - distributed

- a local Go application initates the process, but the chromedp instance is hosted on a fleet remotely, such as a dedicated Cloud Run with chromedp, and the screenshots are stored on a mounted GCS drive
