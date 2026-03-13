// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
