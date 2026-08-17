from PIL import Image
import math

img_path = '/Users/xxww/.gemini/antigravity/brain/da4532d0-acb7-4cb8-927a-099242128d6e/.user_uploaded/media_1786769906670.png'
img = Image.open(img_path).convert('RGB')
w, h = img.size
print(f"Original dimensions: {w} x {h}")

# Find bounding box of non-white pixels
pixels = img.load()
min_x, max_x = w, 0
min_y, max_y = h, 0

for y in range(h):
    for x in range(w):
        r, g, b = pixels[x, y]
        # if not white/light grey background
        if r < 240 or g < 240 or b < 240:
            if x < min_x: min_x = x
            if x > max_x: max_x = x
            if y < min_y: min_y = y
            if y > max_y: max_y = y

print(f"Logo bbox: x=[{min_x}, {max_x}] ({max_x - min_x} px), y=[{min_y}, {max_y}] ({max_y - min_y} px)")

# Let's sample colors across the logo to understand the exact structure
logo_w = max_x - min_x
logo_h = max_y - min_y

# Sample points relative to bbox (0.0 to 1.0)
sample_points = [
    ("Top Peak", 0.5, 0.05),
    ("Top Left Roof", 0.35, 0.15),
    ("Top Right Roof", 0.65, 0.15),
    ("Upper Left Outer", 0.05, 0.25),
    ("Upper Left Inner", 0.25, 0.25),
    ("Stem Middle", 0.1, 0.5),
    ("Stem Bottom", 0.1, 0.8),
    ("Inner Hole Center", 0.5, 0.35),
    ("Upper Loop Right Edge", 0.95, 0.35),
    ("Waist Center", 0.5, 0.5),
    ("Leg Top Light", 0.6, 0.58),
    ("Leg Middle Fold", 0.65, 0.7),
    ("Leg Bottom Dark", 0.5, 0.72),
    ("Leg End Tip", 0.85, 0.78),
]

print("\n--- Color Samples ---")
for name, rx, ry in sample_points:
    px = int(min_x + rx * logo_w)
    py = int(min_y + ry * logo_h)
    if 0 <= px < w and 0 <= py < h:
        r, g, b = pixels[px, py]
        hex_col = f"#{r:02X}{g:02X}{b:02X}"
        print(f"{name:22s} @ ({rx:.2f}, {ry:.2f}) -> {hex_col} (RGB: {r},{g},{b})")
