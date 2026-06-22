package service

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // 注册 JPEG 解码器，支持未来真模型返回 jpg 背景
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

// PosterComposer 把背景图 + 标题合成为最终海报 PNG。
// 字体在 NewPosterComposer 启动时一次性加载；缺失则降级（仅输出无文字背景图）。
type PosterComposer struct {
	font *truetype.Font // nil 表示字体加载失败，合成时跳过文字
}

func NewPosterComposer(fontPath string) *PosterComposer {
	pc := &PosterComposer{}
	data, err := os.ReadFile(fontPath)
	if err != nil {
		log.Printf("[poster] font not loaded, will skip title text: %v", err)
		return pc
	}
	f, err := truetype.Parse(data)
	if err != nil {
		log.Printf("[poster] font parse failed, will skip title text: %v", err)
		return pc
	}
	pc.font = f
	return pc
}

// Compose 把 bgPath 加载为底图，把 title 画在底部居中（白色 + 黑色描边），输出到 outPath。
// title 为空 / 字体未加载时只输出原始背景图。
func (p *PosterComposer) Compose(bgPath, title, outPath string) error {
	bg, err := loadImage(bgPath)
	if err != nil {
		return fmt.Errorf("load bg: %w", err)
	}
	rgba := image.NewRGBA(bg.Bounds())
	draw.Draw(rgba, rgba.Bounds(), bg, image.Point{}, draw.Src)

	if p.font != nil && title != "" {
		if err := p.drawTitleBottomCenter(rgba, title); err != nil {
			return fmt.Errorf("draw title: %w", err)
		}
	}

	return writePNGFile(outPath, rgba)
}

// drawTitleBottomCenter 在 dst 底部 1/6 区域内居中画 title。
// 字号按图片宽度自适应（取宽度的 6%），用白色填充 + 黑色描边提升对比度。
func (p *PosterComposer) drawTitleBottomCenter(dst *image.RGBA, title string) error {
	bounds := dst.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	fontSize := float64(w) * 0.06
	if fontSize < 24 {
		fontSize = 24
	}

	// 测量文字宽度，决定起笔 X
	face := truetype.NewFace(p.font, &truetype.Options{Size: fontSize, DPI: 72, Hinting: font.HintingFull})
	defer face.Close()

	textWidth := font.MeasureString(face, title).Round()
	x := (w - textWidth) / 2
	y := h - h/12 // 底部留 1/12 边距

	// 描边：在 8 个方向各画一遍黑色，再在中间画白色
	drawText := func(c *freetype.Context, col color.Color, dx, dy int) error {
		c.SetSrc(image.NewUniform(col))
		_, err := c.DrawString(title, freetype.Pt(x+dx, y+dy))
		return err
	}

	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(p.font)
	c.SetFontSize(fontSize)
	c.SetClip(dst.Bounds())
	c.SetDst(dst)
	c.SetHinting(font.HintingFull)

	stroke := int(fontSize / 16)
	if stroke < 2 {
		stroke = 2
	}
	for dx := -stroke; dx <= stroke; dx++ {
		for dy := -stroke; dy <= stroke; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}
			if err := drawText(c, color.Black, dx, dy); err != nil {
				return err
			}
		}
	}
	if err := drawText(c, color.White, 0, 0); err != nil {
		return err
	}
	return nil
}

// loadImage 支持 PNG / JPEG（依靠 image 包注册的解码器）。
func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return img, nil
}

func writePNGFile(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer f.Close()
	return png.Encode(f, img)
}
