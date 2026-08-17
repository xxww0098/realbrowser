from PIL import Image

img_path = '/Users/xxww/.gemini/antigravity/brain/da4532d0-acb7-4cb8-927a-099242128d6e/.user_uploaded/media_1786769906670.png'
img = Image.open(img_path).convert('RGB')
w, h = img.size
pixels = img.load()

# Let's upscale or print the exact RGB values for every single pixel in the logo box
print("=== EXACT RGB MAP OF EACH PIXEL ===")
with open("/Volumes/Acasis/Code/REPO/realbrowser/.scratch/rgb_map.txt", "w") as f:
    for y in range(26, 84):
        f.write(f"--- ROW Y={y} ---\n")
        row_str = []
        for x in range(23, 76):
            r, g, b = pixels[x, y]
            row_str.append(f"({x:2d},{y:2d}):#{r:02x}{g:02x}{b:02x} ")
        f.write("".join(row_str) + "\n")

print("RGB map dumped to .scratch/rgb_map.txt")
