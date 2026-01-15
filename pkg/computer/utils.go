package computer

import (
	"context"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func captureFullPageScreenshot(ctx context.Context, res *[]byte) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, contentSize, _, _, _, err := page.GetLayoutMetrics().Do(ctx)
		if err != nil {
			return err
		}
		width, height := int64(contentSize.Width), int64(contentSize.Height)
		return chromedp.Run(ctx,
			chromedp.EmulateViewport(width, height),
			chromedp.CaptureScreenshot(res),
			chromedp.EmulateViewport(1280, 1024),
		)
	}))
}
