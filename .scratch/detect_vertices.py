from PIL import Image

img_path = '/Users/xxww/.gemini/antigravity/brain/da4532d0-acb7-4cb8-927a-099242128d6e/.user_uploaded/media_1786769906670.png'
img = Image.open(img_path).convert('RGB')
w, h = img.size
pixels = img.load()

# Let's crop and inspect the exact logo area
# Bounding box around the blue logo
# Let's find exact coordinates of all corners and vertices:
# 1. Top Apex Peak
# 2. Top-Left Outer Shoulder
# 3. Top-Right Outer Shoulder
# 4. Rightmost Loop Outer Vertical Edge
# 5. Right Loop Inward Angle Point
# 6. Bottom-Left Outer Corner
# 7. Bottom-Left Inner Chamfer Point
# 8. Inner Hole Peak
# 9. Inner Hole Left Vertical
# 10. Inner Hole Right Vertical
# 11. Inner Hole Bottom Angle
# 12. Leg Top Start
# 13. Leg Outer Top Tip
# 14. Leg Outer Bottom Tip
# 15. Leg Inner Bottom Point

# Let's scan and print specific slices with RGB values
print("--- VERTEX ANALYSIS ---")

# Find Top Peak
for y in range(h):
    row_blue = [(x, pixels[x, y]) for x in range(w) if pixels[x, y][2] > 200 and pixels[x, y][0] < 200]
    if row_blue:
        xs = [p[0] for p in row_blue]
        print(f"Top Peak y={y}: x_range=[{min(xs)}, {max(xs)}], center_x={sum(xs)/len(xs):.1f}")
        break

# Let's inspect colors of each region:
# Let's extract high-res color samples across all facets
print("\n--- DETAILED REGION COLORS ---")
# Left stem middle
print("Left Stem (x=29, y=65):", pixels[29, 65])
# Left stem top
print("Left Stem Top (x=29, y=45):", pixels[29, 45])
# Left stem bottom
print("Left Stem Bottom (x=29, y=75):", pixels[29, 75])
# Roof Left (x=38, y=38):
print("Roof Left (x=38, y=38):", pixels[38, 38])
# Roof Peak (x=53, y=31):
print("Roof Peak (x=53, y=31):", pixels[53, 31])
# Roof Right (x=68, y=38):
print("Roof Right (x=68, y=38):", pixels[68, 38])
# Right Loop Outer (x=73, y=50):
print("Right Loop Outer (x=73, y=50):", pixels[73, 50])
# Inner Hole (x=53, y=46):
print("Inner Hole (x=53, y=46):", pixels[53, 46])
# Leg Top Surface (x=68, y=68):
print("Leg Top Surface (x=68, y=68):", pixels[68, 68])
# Leg Bottom / Underside (x=55, y=70):
print("Leg Bottom / Underside (x=55, y=70):", pixels[55, 70])
# Leg Tip (x=73, y=75):
print("Leg Tip (x=73, y=75):", pixels[73, 75])
