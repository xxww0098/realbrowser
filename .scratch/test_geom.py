import math
import os

def generate_faceted_r():
    # Base dimensions
    width = 512
    height = 512
    
    # We will define the nodes on a precise geometric coordinate system
    # Hexagonal angles: 30 deg / 60 deg / 90 deg / 120 deg / 150 deg
    
    # Center of figure
    cx = 256
    cy = 246  # slight vertical centering offset
    
    # Geometry parameters
    # Let R be the outer radius of the hexagon
    R = 210
    # cos(30) = 0.866025, sin(30) = 0.5
    cos30 = math.cos(math.radians(30))
    sin30 = math.sin(math.radians(30))
    
    # Key outer points
    # P_TOP: Peak (top vertex)
    P_TOP = (cx, cy - R) # (256, 36)
    
    # P_TR: Top Right corner
    P_TR = (cx + R * cos30, cy - R * sin30) # (256 + 181.86 = 437.86, 246 - 105 = 141)
    
    # P_TL: Top Left corner
    P_TL = (cx - R * cos30, cy - R * sin30) # (256 - 181.86 = 74.14, 141)
    
    # P_BL: Bottom Left corner of stem
    P_BL = (cx - R * cos30, cy + R * sin30 + 50) # (74.14, 401)
    
    # P_BL_IN: Chamfer point of bottom left stem (slanted up-right at 30 deg)
    stem_w = 64
    P_BL_IN = (P_BL[0] + stem_w, P_BL[1] - stem_w * sin30 / cos30)
    
    print("Computed points:", P_TOP, P_TR, P_TL)

generate_faceted_r()
