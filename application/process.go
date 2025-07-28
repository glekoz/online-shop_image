package application

import (
	"context"
	"image"
	"image/color"
)

// Добавить контекст???
func toGrayScale(ctx context.Context, img image.Image) (image.Image, error) {
	bounds := img.Bounds()
	grayImg := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		if y%500 == 0 {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			originalColor := img.At(x, y)
			grayColor := color.GrayModel.Convert(originalColor)
			grayImg.Set(x, y, grayColor)
		}
	}
	return grayImg, nil
}
