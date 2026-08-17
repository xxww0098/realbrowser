from PIL import Image

img_path = '/Users/xxww/.gemini/antigravity/brain/da4532d0-acb7-4cb8-927a-099242128d6e/.user_uploaded/media_1786769906670.png'
img = Image.open(img_path).convert('RGB')
w, h = img.size
pixels = img.load()

# Let's print a downscaled or exact ASCII representation showing brightness and hue
print(f"Image Size: {w} x {h}")

# Find actual logo boundaries
# Classify each pixel:
# ' ' = background (white/near white, r>240, g>240, b>240)
# Letters / characters based on brightness and blue intensity:
# '.' light blue / highlight (r>100, b>220)
# '*' bright blue (r < 60, b > 230, g > 100)
# '#' deep blue (b > 180, r < 50, g < 100)
# '@' dark blue (b < 180)

grid = []
for y in range(0, h, 2):
    row = []
    for x in range(0, w, 1):
        r, g, b = pixels[x, y]
        if r > 240 and g > 240 and b > 240:
            row.append(' ')
        elif r > 100 and b > 200:
            row.append('.') # highlight
        elif b > 230 and g > 100:
            row.append('+') # light-medium blue
        elif b > 200 and g > 60:
            row.append('*') # bright blue
        elif b > 150:
            row.append('#') # deep blue
        else:
            row.append('@') # darkest blue
    grid.append("".join(row))

print("\n".join(grid))
