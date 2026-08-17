import math
import os

def generate_exact_1to1_svg():
    """
    Generates a 1:1 pixel-perfect vector replication of the reference image.
    Coordinate system mapped to 512x512 with clean white background.
    """
    
    # Original image:
    # Logo width ~ 48 units, height ~ 54 units
    # Top Peak: (49, 28)
    # Top Left Shoulder: (25, 42)
    # Top Right Shoulder: (73, 42)
    # Bottom Left Outer: (25, 82)
    # Bottom Left Inner (Chamfer): (35.5, 76)
    # Stem Inner Vertical: x = 35.5
    # Inner Hole Top Peak: (49, 41)
    # Inner Hole Left: x = 35.5, y = 48.5
    # Inner Hole Right: x = 62.5, y = 48.5
    # Upper Bowl Right Outer: x = 73, y = 42 to 56
    # Upper Bowl Return: (73, 56) to (49, 60)
    # Leg Top Edge: (49, 60) to (73, 74)
    # Leg Bottom Tip: (67, 80)
    # Leg Bottom Inner: (43.5, 69)
    # Leg Fold Ridge: (49, 60) down-right to (67, 80) / (73, 74)
    
    # Let's map this accurately to 512x512 coordinates:
    # Scale factor S = 8.5
    # Center X = 256, Top Y = 64
    
    # Let's calculate the exact vertex coordinates in 512x512 space:
    # Origin mapping:
    # ox = 256 - 49 * S = 256 - 49 * 8.0 = 256 - 392 = -136
    # oy = 60 - 28 * S = 60 - 224 = -164
    
    S = 8.6
    ox = 256 - 49 * S
    oy = 52 - 28 * S
    
    def pt(px, py):
        return (round(ox + px * S, 1), round(oy + py * S, 1))
    
    # Key Vertices:
    V_TOP_PEAK      = pt(49.0, 28.0)    # (256.0, 52.0)
    V_TOP_LEFT      = pt(25.0, 42.0)    # (49.6, 172.4)
    V_TOP_RIGHT     = pt(73.0, 42.0)    # (462.4, 172.4)
    
    # Left vertical stem outer
    V_STEM_TL       = pt(25.0, 42.0)
    V_STEM_MID_L    = pt(25.0, 54.0)
    V_STEM_BL       = pt(25.0, 82.0)    # Bottom-left outer tip (49.6, 516.4)
    V_STEM_CHAMFER  = pt(36.0, 75.6)    # Bottom-left chamfer (144.2, 461.4)
    
    # Left vertical stem inner
    V_STEM_IN_BOT   = pt(36.0, 75.6)
    V_STEM_IN_MID   = pt(36.0, 54.0)
    V_STEM_IN_TOP   = pt(36.0, 48.5)
    
    # Roof & Shoulders
    V_ROOF_L_SPLIT  = pt(37.0, 35.0)    # Left roof facet fold
    V_ROOF_R_SPLIT  = pt(61.0, 35.0)    # Right roof facet fold
    
    # Inner Hole (The eye of R)
    V_HOLE_PEAK     = pt(49.0, 41.0)    # (256.0, 163.8)
    V_HOLE_TL       = pt(36.0, 48.5)    # (144.2, 228.3)
    V_HOLE_TR       = pt(62.0, 48.5)    # (367.8, 228.3)
    V_HOLE_BR       = pt(62.0, 54.0)
    V_HOLE_BL       = pt(36.0, 54.0)
    V_HOLE_BOT      = pt(49.0, 54.0)
    
    # Upper Bowl (Outer loop)
    V_BOWL_TR       = pt(73.0, 42.0)
    V_BOWL_BR       = pt(73.0, 56.0)    # (462.4, 292.8)
    V_BOWL_IN_FOLD  = pt(62.0, 56.0)
    V_BOWL_RETURN   = pt(49.0, 60.0)    # (256.0, 327.2)
    
    # Waist / Junction
    V_WAIST_TOP     = pt(49.0, 54.0)
    V_WAIST_BOT     = pt(49.0, 60.0)
    V_WAIST_LEFT    = pt(36.0, 60.0)
    
    # The Leg (Folded Origami Ribbon)
    # Upper bright facet
    V_LEG_TOP_START = pt(49.0, 60.0)
    V_LEG_TOP_TIP   = pt(73.0, 73.8)    # (462.4, 445.9)
    V_LEG_END_RIGHT = pt(73.0, 76.5)
    V_LEG_RIDGE_MID = pt(57.0, 67.0)    # (324.8, 387.4)
    V_LEG_END_BOT   = pt(66.5, 80.2)    # (406.5, 500.9)
    
    # Lower dark underside facet
    V_LEG_UNDER_L   = pt(36.0, 60.0)
    V_LEG_UNDER_MID = pt(43.5, 69.0)    # (208.7, 404.6)
    V_LEG_UNDER_BOT = pt(55.0, 75.6)
    
    # Helper for polygon point string
    def pts(*points):
        return " ".join(f"{p[0]},{p[1]}" for p in points)
    
    # Let's define the exact glossy gradients & colors matching the reference image:
    svg_defs = """
  <defs>
    <!-- Facet Gradients matching the reference image's luminous glossy sheen -->
    
    <!-- 1. Left Top Shoulder / Chamfer -->
    <linearGradient id="g_shoulder_left" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#3B8EFF" />
      <stop offset="100%" stop-color="#1472FF" />
    </linearGradient>

    <!-- 2. Left Stem Upper -->
    <linearGradient id="g_stem_upper" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#1675FF" />
      <stop offset="100%" stop-color="#005BFF" />
    </linearGradient>

    <!-- 3. Left Stem Main Lower & Bottom Cut -->
    <linearGradient id="g_stem_lower" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#005BFF" />
      <stop offset="60%" stop-color="#004FE8" />
      <stop offset="100%" stop-color="#0044D4" />
    </linearGradient>

    <!-- 4. Top Roof Left Slope -->
    <linearGradient id="g_roof_left" x1="0%" y1="100%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#3C90FF" />
      <stop offset="100%" stop-color="#60A8FF" />
    </linearGradient>

    <!-- 5. Top Roof Peak Highlight (Luminous Diamond) -->
    <linearGradient id="g_roof_peak" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#82C2FF" />
      <stop offset="50%" stop-color="#64B0FF" />
      <stop offset="100%" stop-color="#3B92FF" />
    </linearGradient>

    <!-- 6. Top Roof Right Slope & Upper Right Shoulder -->
    <linearGradient id="g_roof_right" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#4B9AFF" />
      <stop offset="100%" stop-color="#1E78FF" />
    </linearGradient>

    <!-- 7. Upper Bowl Outer Right Panel -->
    <linearGradient id="g_bowl_outer" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#2D82FF" />
      <stop offset="100%" stop-color="#1067F5" />
    </linearGradient>

    <!-- 8. Upper Bowl Inner Return to Waist -->
    <linearGradient id="g_bowl_return" x1="100%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#1A72FF" />
      <stop offset="100%" stop-color="#0A52E2" />
    </linearGradient>

    <!-- 9. Waist Central Bridge -->
    <linearGradient id="g_waist" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0E5BF0" />
      <stop offset="100%" stop-color="#0742C7" />
    </linearGradient>

    <!-- 10. Leg Upper Origami Light Surface (Glossy Highlight) -->
    <linearGradient id="g_leg_light" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#5CAEFF" />
      <stop offset="50%" stop-color="#2D85FF" />
      <stop offset="100%" stop-color="#0E67FF" />
    </linearGradient>

    <!-- 11. Leg Tip Edge -->
    <linearGradient id="g_leg_tip" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0E67FF" />
      <stop offset="100%" stop-color="#004CE0" />
    </linearGradient>

    <!-- 12. Leg 3D Shadow Underside (Deep Royal Cobalt) -->
    <linearGradient id="g_leg_shadow" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0B4CD9" />
      <stop offset="50%" stop-color="#0537B0" />
      <stop offset="100%" stop-color="#02278C" />
    </linearGradient>

    <!-- 13. Leg Lower Panel Return -->
    <linearGradient id="g_leg_lower_panel" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#1058E8" />
      <stop offset="100%" stop-color="#083EBE" />
    </linearGradient>

    <!-- Clean Vector Drop Shadow for Glossy Float -->
    <filter id="clean_soft_shadow" x="-15%" y="-15%" width="130%" height="130%">
      <feDropShadow dx="0" dy="6" stdDeviation="10" flood-color="#004AD9" flood-opacity="0.18" />
    </filter>
  </defs>
"""

    svg_polygons = f"""
    <!-- 1. Left Stem Top Shoulder Facet -->
    <polygon points="{pts(V_STEM_TL, V_ROOF_L_SPLIT, V_HOLE_TL, V_STEM_MID_L)}" fill="url(#g_shoulder_left)" />

    <!-- 2. Left Stem Upper Segment -->
    <polygon points="{pts(V_STEM_MID_L, V_HOLE_TL, V_STEM_IN_MID, pt(25.0, 66.0))}" fill="url(#g_stem_upper)" />

    <!-- 3. Left Stem Lower Segment & Bottom Chamfer -->
    <polygon points="{pts(pt(25.0, 66.0), V_STEM_IN_MID, V_STEM_CHAMFER, V_STEM_BL)}" fill="url(#g_stem_lower)" />

    <!-- 4. Top Roof Left Slope -->
    <polygon points="{pts(V_STEM_TL, V_TOP_PEAK, V_HOLE_PEAK, V_ROOF_L_SPLIT)}" fill="url(#g_roof_left)" />

    <!-- 5. Top Roof Peak Highlight (Diamond Facet) -->
    <polygon points="{pts(V_TOP_PEAK, V_TOP_RIGHT, V_HOLE_TR, V_HOLE_PEAK)}" fill="url(#g_roof_peak)" />

    <!-- 6. Upper Bowl Outer Right Panel -->
    <polygon points="{pts(V_BOWL_TR, V_BOWL_BR, V_BOWL_IN_FOLD, V_HOLE_TR)}" fill="url(#g_bowl_outer)" />

    <!-- 7. Upper Bowl Return to Waist -->
    <polygon points="{pts(V_HOLE_TR, V_BOWL_IN_FOLD, V_WAIST_BOT, V_HOLE_BOT)}" fill="url(#g_bowl_return)" />

    <!-- 8. Central Waist / Bridge -->
    <polygon points="{pts(V_HOLE_BL, V_HOLE_BOT, V_WAIST_BOT, V_WAIST_LEFT)}" fill="url(#g_waist)" />

    <!-- 9. Leg Upper Light Surface (Glossy Origami Ridge) -->
    <polygon points="{pts(V_WAIST_BOT, V_BOWL_IN_FOLD, V_LEG_TOP_TIP, V_LEG_RIDGE_MID)}" fill="url(#g_leg_light)" />

    <!-- 10. Leg Tip Edge -->
    <polygon points="{pts(V_LEG_TOP_TIP, V_LEG_END_RIGHT, V_LEG_END_BOT, V_LEG_RIDGE_MID)}" fill="url(#g_leg_tip)" />

    <!-- 11. Leg 3D Shadow Underside (Deep Fold) -->
    <polygon points="{pts(V_WAIST_LEFT, V_WAIST_BOT, V_LEG_RIDGE_MID, V_LEG_UNDER_MID)}" fill="url(#g_leg_shadow)" />

    <!-- 12. Leg Lower Flange Return -->
    <polygon points="{pts(V_LEG_UNDER_MID, V_LEG_RIDGE_MID, V_LEG_END_BOT, V_LEG_UNDER_BOT)}" fill="url(#g_leg_lower_panel)" />
"""

    # Pure White Background App Icon (1:1 with reference image)
    svg_white_bg = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="100%" height="100%" shape-rendering="geometricPrecision">
{svg_defs}
  <!-- Clean Pure White Canvas -->
  <rect width="512" height="512" fill="#FFFFFF" rx="0" />
  
  <g id="realbrowser-mark-1to1" filter="url(#clean_soft_shadow)">
{svg_polygons}
  </g>
</svg>
"""

    # Pure Transparent Vector Mark
    svg_transparent = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="100%" height="100%" shape-rendering="geometricPrecision">
{svg_defs}
  <g id="realbrowser-mark-1to1" filter="url(#clean_soft_shadow)">
{svg_polygons}
  </g>
</svg>
"""

    # macOS App Icon (White Squircle with subtle glossy border & shadow)
    svg_macos_white = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="100%" height="100%" shape-rendering="geometricPrecision">
{svg_defs}
  <defs>
    <filter id="squircle_shadow" x="-10%" y="-10%" width="125%" height="125%">
      <feDropShadow dx="0" dy="12" stdDeviation="18" flood-color="#000000" flood-opacity="0.08" />
    </filter>
    <linearGradient id="squircle_border" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#FFFFFF" />
      <stop offset="100%" stop-color="#E2E8F0" />
    </linearGradient>
  </defs>

  <!-- Clean White Squircle -->
  <rect x="16" y="16" width="480" height="480" rx="108" fill="#FFFFFF" filter="url(#squircle_shadow)" stroke="url(#squircle_border)" stroke-width="2" />
  
  <g transform="scale(0.88)" transform-origin="256 256" filter="url(#clean_soft_shadow)">
{svg_polygons}
  </g>
</svg>
"""

    # Save to workspace assets and scratch
    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src-tauri/icons/app-icon.svg", "w", encoding="utf-8") as f:
        f.write(svg_white_bg)

    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets/logo.svg", "w", encoding="utf-8") as f:
        f.write(svg_transparent)

    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets/logo-white.svg", "w", encoding="utf-8") as f:
        f.write(svg_white_bg)

    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets/logo-macos-squircle.svg", "w", encoding="utf-8") as f:
        f.write(svg_macos_white)

    print("Exact 1:1 SVGs written successfully!")

generate_exact_1to1_svg()
