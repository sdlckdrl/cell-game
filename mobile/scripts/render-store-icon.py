from pathlib import Path
from PIL import Image, ImageDraw, ImageFilter


ROOT = Path(__file__).resolve().parents[1]
OUT_DIR = ROOT / "store-assets" / "google-play"
ICON_SIZE = 1024


def radial_fill(base, bbox, inner_rgba, outer_rgba, blur=0):
    layer = Image.new("RGBA", base.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(layer)
    left, top, right, bottom = bbox
    width = right - left
    height = bottom - top
    steps = int(max(width, height) / 2)
    for step in range(steps, 0, -1):
        t = step / steps
        rgba = tuple(
            int(inner_rgba[i] * t + outer_rgba[i] * (1 - t))
            for i in range(4)
        )
        inset_x = width * (1 - t) * 0.5
        inset_y = height * (1 - t) * 0.5
        draw.ellipse(
            (
                left + inset_x,
                top + inset_y,
                right - inset_x,
                bottom - inset_y,
            ),
            fill=rgba,
        )
    if blur:
        layer = layer.filter(ImageFilter.GaussianBlur(blur))
    base.alpha_composite(layer)


def ring(base, bbox, color, width, blur=0):
    layer = Image.new("RGBA", base.size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(layer)
    draw.ellipse(bbox, outline=color, width=width)
    if blur:
        layer = layer.filter(ImageFilter.GaussianBlur(blur))
    base.alpha_composite(layer)


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)

    image = Image.new("RGBA", (ICON_SIZE, ICON_SIZE), (7, 17, 31, 255))

    radial_fill(
        image,
        (90, 90, 934, 934),
        (20, 54, 91, 255),
        (3, 8, 18, 255),
        blur=16,
    )
    radial_fill(
        image,
        (180, 140, 920, 880),
        (32, 94, 139, 82),
        (7, 17, 31, 0),
        blur=28,
    )

    radial_fill(
        image,
        (248, 248, 776, 776),
        (118, 245, 255, 220),
        (27, 112, 180, 36),
        blur=28,
    )
    ring(image, (246, 246, 778, 778), (164, 238, 255, 192), 10, blur=1)
    ring(image, (208, 208, 816, 816), (94, 170, 255, 72), 8, blur=10)
    ring(image, (172, 172, 852, 852), (82, 255, 207, 50), 8, blur=18)

    radial_fill(
        image,
        (302, 286, 712, 696),
        (221, 255, 255, 255),
        (71, 211, 255, 160),
        blur=6,
    )
    radial_fill(
        image,
        (342, 330, 680, 668),
        (164, 255, 247, 255),
        (34, 140, 225, 90),
        blur=8,
    )
    radial_fill(
        image,
        (404, 394, 616, 606),
        (255, 255, 255, 250),
        (151, 255, 249, 150),
        blur=10,
    )

    glow = Image.new("RGBA", image.size, (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)
    glow_draw.ellipse((358, 324, 500, 466), fill=(255, 255, 255, 84))
    glow_draw.ellipse((690, 270, 780, 360), fill=(164, 255, 247, 120))
    glow_draw.ellipse((260, 670, 326, 736), fill=(164, 255, 247, 90))
    glow = glow.filter(ImageFilter.GaussianBlur(22))
    image.alpha_composite(glow)

    accent = Image.new("RGBA", image.size, (0, 0, 0, 0))
    accent_draw = ImageDraw.Draw(accent)
    accent_draw.ellipse((700, 280, 770, 350), fill=(138, 255, 207, 255))
    accent_draw.ellipse((282, 690, 318, 726), fill=(96, 185, 255, 230))
    accent = accent.filter(ImageFilter.GaussianBlur(1))
    image.alpha_composite(accent)

    path_1024 = OUT_DIR / "play-icon-1024.png"
    image.save(path_1024, format="PNG")

    image.resize((512, 512), Image.Resampling.LANCZOS).save(
        OUT_DIR / "play-icon-512.png",
        format="PNG",
    )


if __name__ == "__main__":
    main()
