from PIL import Image

img_path = '/Users/xxww/.gemini/antigravity/brain/da4532d0-acb7-4cb8-927a-099242128d6e/.user_uploaded/media_1786769906670.png'
img = Image.open(img_path).convert('RGB')
w, h = img.size
pixels = img.load()

# Let's print out the exact RGB values around all key coordinates in a cropped 60x60 grid
# Let's find the bounding box of the blue R
min_x, max_x, min_y, max_y = 100, 0, 100, 0
for y in range(h):
    for x in range(w):
        r, g, b = pixels[x, y]
        # blue pixel
        if b > 160 and (b > r + 30):
            min_x = min(min_x, x)
            max_x = max(max_x, x)
            min_y = min(min_y, y)
            max_y = max(max_y, y)

print(f"Exact Blue R Box: x=[{min_x}, {max_x}] (w={max_x - min_x + 1}), y=[{min_y}, {max_y}] (h={max_y - min_y + 1})")

# Let's inspect the facets within this box:
# Print a color table of the logo region
for y in range(min_y, max_y + 1):
    row_chars = []
    for x in range(min_x, max_x + 1):
        r, g, b = pixels[x, y]
        if b < 160 or (b <= r + 20):
            row_chars.append("  ") # white space
        else:
            # classify color shade:
            # 1: Very light highlight cyan/sky (r>60, g>140, b>240)
            # 2: Light blue (r>30, g>110, b>240)
            # 3: Medium bright blue (r<30, g>80, b>240)
            # 4: Deep blue (g<80, b>200)
            # 5: Dark shadow (b<200)
            if r > 60 and g > 140:
                row_chars.append("11")
            elif r > 20 and g > 110:
                row_chars.append("22")
            elif g > 80:
                row_chars.append("33")
            elif g > 50:
                row_chars.append("44")
            else:
                row_chars.append("55")
    print(f"{y:2d}: " + "".join(row_chars))
