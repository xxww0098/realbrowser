from PIL import Image

img_path = '/Users/xxww/.gemini/antigravity/brain/da4532d0-acb7-4cb8-927a-099242128d6e/.user_uploaded/media_1786769906670.png'
img = Image.open(img_path).convert('RGB')
pixels = img.load()

# Let's inspect the exact RGB across the key regions
# 1. Left stem top-left triangle / polygon:
print("Left shoulder (25, 42):", pixels[25, 42])
print("Left shoulder inner (30, 42):", pixels[30, 42])
print("Stem upper (27, 48):", pixels[27, 48])
print("Stem middle (27, 60):", pixels[27, 60])
print("Stem lower (27, 72):", pixels[27, 72])
print("Stem bottom tip (25, 82):", pixels[25, 82])
print("Stem bottom inner (35, 76):", pixels[35, 76])

# 2. Roof:
print("\nRoof left slope (37, 35):", pixels[37, 35])
print("Roof peak top (49, 28):", pixels[49, 28])
print("Roof peak lower (49, 36):", pixels[49, 36])
print("Roof right slope (61, 35):", pixels[61, 35])
print("Roof right shoulder (73, 42):", pixels[73, 42])

# 3. Upper Bowl:
print("\nUpper bowl outer right (73, 48):", pixels[73, 48])
print("Upper bowl inner right (67, 48):", pixels[67, 48])
print("Upper bowl bottom right (73, 56):", pixels[73, 56])
print("Upper bowl bottom return (60, 58):", pixels[60, 58])

# 4. Inner Hole:
print("\nInner hole top (49, 41):", pixels[49, 41])
print("Inner hole left (36, 50):", pixels[36, 50])
print("Inner hole right (62, 50):", pixels[62, 50])

# 5. Leg:
print("\nLeg top corner (47, 54):", pixels[47, 54])
print("Leg upper light surface (60, 64):", pixels[60, 64])
print("Leg top tip (73, 74):", pixels[73, 74])
print("Leg fold ridge (55, 68):", pixels[55, 68])
print("Leg dark underside (48, 70):", pixels[48, 70])
print("Leg bottom tip (67, 80):", pixels[67, 80])
print("Leg bottom inner (44, 69):", pixels[44, 69])
