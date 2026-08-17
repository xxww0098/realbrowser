import math
import os

def generate_svg_suite():
    # -------------------------------------------------------------
    # Geometry Definition for 512x512
    # Center: (256, 256)
    # Hexagon outer radius ~ 200
    # -------------------------------------------------------------
    
    # 1. Primary Origami Faceted Model Points:
    # Top Peak
    P_PEAK = (256.0, 52.0)
    
    # Roof Upper Slopes
    P_ROOF_L_MID = (180.0, 96.0)
    P_ROOF_R_MID = (332.0, 96.0)
    
    # Outer Hexagon Corners
    P_TL_OUT = (104.0, 140.0)      # Top Left Outer
    P_TR_OUT = (408.0, 140.0)      # Top Right Outer
    P_BR_BOWL = (408.0, 272.0)     # Upper Bowl Bottom Right Outer
    
    # Left Vertical Stem Outer
    P_BL_OUT = (104.0, 396.0)      # Bottom Left Outer
    P_BL_CHAMFER = (174.0, 355.0)  # Bottom Left Chamfered Tip
    
    # Left Stem Inner
    P_STEM_IN_BOT = (174.0, 355.0)
    P_STEM_IN_MID = (174.0, 260.0)
    P_STEM_IN_TOP = (174.0, 180.0)
    
    # Left Stem Seams (for faceted triangles)
    P_STEM_SEAM_L = (104.0, 180.0)
    P_STEM_SEAM_MID = (104.0, 280.0)
    
    # Counter Hole (The geometric eye of the R)
    P_HOLE_PEAK = (256.0, 128.0)
    P_HOLE_TL   = (174.0, 175.0)
    P_HOLE_TR   = (338.0, 175.0)
    P_HOLE_BR   = (338.0, 240.0)
    P_HOLE_BOT  = (256.0, 240.0)
    P_HOLE_BL   = (174.0, 240.0)
    
    # Upper Bowl Inner Fold
    P_BOWL_IN_FOLD = (338.0, 272.0)
    
    # Central Waist / Bridge
    P_WAIST_CEN = (256.0, 240.0)
    P_WAIST_BOT = (256.0, 288.0)
    P_WAIST_LEFT = (174.0, 288.0)
    
    # The Leg (Origami Folded Ribbon)
    # Upper Leg Light Surface:
    P_LEG_START = (256.0, 288.0)
    P_LEG_MID_UPPER = (338.0, 272.0)
    P_LEG_TIP_TOP = (408.0, 355.0)
    P_LEG_TIP_RIGHT = (408.0, 396.0)
    P_LEG_RIDGE_MID = (326.0, 348.0)
    
    # Lower Leg Shadow Underside:
    P_LEG_SHADOW_LEFT = (174.0, 288.0)
    P_LEG_SHADOW_CEN = (256.0, 335.0)
    P_LEG_RIDGE_BOT = (326.0, 395.0)
    P_LEG_TIP_BOT = (256.0, 435.0)
    
    # Helper to generate polygon svg element
    def poly(points, fill, opacity=1.0, stroke="none", stroke_w=0):
        pts = " ".join(f"{round(p[0], 1)},{round(p[1], 1)}" for p in points)
        st = f' stroke="{stroke}" stroke-width="{stroke_w}" stroke-linejoin="round"' if stroke != "none" and stroke_w > 0 else ''
        op = f' opacity="{opacity}"' if opacity < 1.0 else ''
        return f'    <polygon points="{pts}" fill="{fill}"{op}{st} />'
    
    # Shared Facet Gradients definition
    shared_gradients = """    <!-- Facet Gradients for Luminous Depth -->
    <linearGradient id="g_stem_top" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#1E6BFF" />
      <stop offset="100%" stop-color="#0052FF" />
    </linearGradient>
    
    <linearGradient id="g_stem_main" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0066FF" />
      <stop offset="100%" stop-color="#0047E0" />
    </linearGradient>
    
    <linearGradient id="g_stem_bot" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0052FF" />
      <stop offset="100%" stop-color="#0035B8" />
    </linearGradient>

    <linearGradient id="g_roof_left" x1="0%" y1="100%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#2D79FF" />
      <stop offset="100%" stop-color="#4B92FF" />
    </linearGradient>

    <linearGradient id="g_roof_peak" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#70B4FF" />
      <stop offset="50%" stop-color="#4B96FF" />
      <stop offset="100%" stop-color="#2575FC" />
    </linearGradient>

    <linearGradient id="g_bowl_outer" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#2B78FF" />
      <stop offset="100%" stop-color="#0F54E8" />
    </linearGradient>

    <linearGradient id="g_bowl_fold" x1="100%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#145BF5" />
      <stop offset="100%" stop-color="#083EBD" />
    </linearGradient>

    <linearGradient id="g_waist" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0B48D1" />
      <stop offset="100%" stop-color="#062E9E" />
    </linearGradient>

    <linearGradient id="g_leg_light" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#4899FF" />
      <stop offset="60%" stop-color="#1E70FF" />
      <stop offset="100%" stop-color="#0055FF" />
    </linearGradient>

    <linearGradient id="g_leg_edge" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0055FF" />
      <stop offset="100%" stop-color="#003DB8" />
    </linearGradient>

    <linearGradient id="g_leg_shadow" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0A3CAE" />
      <stop offset="100%" stop-color="#052473" />
    </linearGradient>

    <linearGradient id="g_leg_bot_panel" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0E48CC" />
      <stop offset="100%" stop-color="#08349E" />
    </linearGradient>

    <!-- Drop Shadow / Ambient Glow Filter -->
    <filter id="mark_glow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="8" stdDeviation="16" flood-color="#0052FF" flood-opacity="0.28" />
    </filter>"""

    def build_polygons():
        lines = []
        # 1. Left stem top corner
        lines.append(poly([P_TL_OUT, P_ROOF_L_MID, P_HOLE_TL, P_STEM_SEAM_L], "url(#g_stem_top)"))
        # 2. Left stem main vertical
        lines.append(poly([P_STEM_SEAM_L, P_HOLE_TL, P_STEM_IN_MID, P_STEM_SEAM_MID], "url(#g_stem_main)"))
        # 3. Left stem lower & chamfer
        lines.append(poly([P_STEM_SEAM_MID, P_STEM_IN_MID, P_BL_CHAMFER, P_BL_OUT], "url(#g_stem_bot)"))
        
        # 4. Top Roof Left slope
        lines.append(poly([P_TL_OUT, P_PEAK, P_HOLE_PEAK, P_HOLE_TL], "url(#g_roof_left)"))
        # 5. Top Roof Peak & Right slope (bright light facet)
        lines.append(poly([P_PEAK, P_TR_OUT, P_HOLE_TR, P_HOLE_PEAK], "url(#g_roof_peak)"))
        
        # 6. Upper Bowl Outer Right
        lines.append(poly([P_TR_OUT, P_BR_BOWL, P_BOWL_IN_FOLD, P_HOLE_TR], "url(#g_bowl_outer)"))
        # 7. Upper Bowl Return to Waist
        lines.append(poly([P_HOLE_TR, P_BOWL_IN_FOLD, P_WAIST_BOT, P_WAIST_CEN], "url(#g_bowl_fold)"))
        
        # 8. Central Waist / Bridge
        lines.append(poly([P_HOLE_BL, P_HOLE_BOT, P_WAIST_BOT, P_WAIST_LEFT], "url(#g_waist)"))
        
        # 9. Leg Upper Surface (Bright Origami Top)
        lines.append(poly([P_WAIST_BOT, P_BOWL_IN_FOLD, P_LEG_TIP_TOP, P_LEG_RIDGE_MID], "url(#g_leg_light)"))
        # 10. Leg Tip Edge Panel
        lines.append(poly([P_LEG_TIP_TOP, P_LEG_TIP_RIGHT, P_LEG_RIDGE_BOT, P_LEG_RIDGE_MID], "url(#g_leg_edge)"))
        # 11. Leg Inner Shadow Underside (Deep 3D Flange)
        lines.append(poly([P_WAIST_LEFT, P_WAIST_BOT, P_LEG_RIDGE_MID, P_LEG_SHADOW_CEN], "url(#g_leg_shadow)"))
        # 12. Leg Bottom Extension Panel
        lines.append(poly([P_LEG_SHADOW_CEN, P_LEG_RIDGE_MID, P_LEG_RIDGE_BOT, P_LEG_TIP_BOT], "url(#g_leg_bot_panel)"))
        
        return "\n".join(lines)

    polys = build_polygons()
    
    # 1. Pure Vector Mark (Transparent)
    svg_mark = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="100%" height="100%" shape-rendering="geometricPrecision">
  <defs>
{shared_gradients}
  </defs>
  <g id="realbrowser-mark" filter="url(#mark_glow)">
{polys}
  </g>
</svg>
"""

    # 2. Modern Desktop App Icon (Squircle container with macOS-grade lighting)
    svg_app_icon = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="100%" height="100%" shape-rendering="geometricPrecision">
  <defs>
{shared_gradients}
    <!-- Container Background Gradients -->
    <linearGradient id="bg_squircle" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#0F172A" />
      <stop offset="50%" stop-color="#0B1120" />
      <stop offset="100%" stop-color="#030712" />
    </linearGradient>

    <!-- App Icon Surface Rim Highlight -->
    <linearGradient id="rim_highlight" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#FFFFFF" stop-opacity="0.16" />
      <stop offset="100%" stop-color="#38BDF8" stop-opacity="0.04" />
    </linearGradient>

    <radialGradient id="ambient_core" cx="50%" cy="40%" r="60%">
      <stop offset="0%" stop-color="#0052FF" stop-opacity="0.25" />
      <stop offset="100%" stop-color="#0052FF" stop-opacity="0" />
    </radialGradient>
  </defs>

  <!-- Squircle Base -->
  <rect x="16" y="16" width="480" height="480" rx="108" fill="url(#bg_squircle)" />
  <rect x="16" y="16" width="480" height="480" rx="108" fill="url(#ambient_core)" />
  <rect x="16" y="16" width="480" height="480" rx="108" fill="none" stroke="url(#rim_highlight)" stroke-width="2" />

  <!-- The Faceted Logo Mark -->
  <g transform="translate(0, 4) scale(0.92)" transform-origin="256 256" filter="url(#mark_glow)">
{polys}
  </g>
</svg>
"""

    # 3. Clean Light Theme App Icon
    svg_light_app_icon = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="100%" height="100%" shape-rendering="geometricPrecision">
  <defs>
{shared_gradients}
    <linearGradient id="bg_light_base" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#FFFFFF" />
      <stop offset="100%" stop-color="#F3F6FC" />
    </linearGradient>
    <radialGradient id="light_core" cx="50%" cy="45%" r="65%">
      <stop offset="0%" stop-color="#0066FF" stop-opacity="0.12" />
      <stop offset="100%" stop-color="#0066FF" stop-opacity="0" />
    </radialGradient>
    <linearGradient id="light_rim" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#FFFFFF" stop-opacity="0.9" />
      <stop offset="100%" stop-color="#CBD5E1" stop-opacity="0.6" />
    </linearGradient>
  </defs>

  <!-- Squircle Base -->
  <rect x="16" y="16" width="480" height="480" rx="108" fill="url(#bg_light_base)" />
  <rect x="16" y="16" width="480" height="480" rx="108" fill="url(#light_core)" />
  <rect x="16" y="16" width="480" height="480" rx="108" fill="none" stroke="url(#light_rim)" stroke-width="2" />

  <!-- Logo Mark -->
  <g transform="translate(0, 4) scale(0.92)" transform-origin="256 256" filter="url(#mark_glow)">
{polys}
  </g>
</svg>
"""

    # 4. Brand Lockup (Logo + RealBrowser Typography)
    svg_brand_lockup = f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 860 256" width="100%" height="100%" shape-rendering="geometricPrecision">
  <defs>
{shared_gradients}
    <linearGradient id="text_grad" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#0F172A" />
      <stop offset="100%" stop-color="#1E293B" />
    </linearGradient>
  </defs>

  <!-- Left Icon Mark -->
  <g transform="translate(10, 8) scale(0.46)" filter="url(#mark_glow)">
{polys}
  </g>

  <!-- Right Typography -->
  <text x="280" y="142" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif" font-weight="800" font-size="78" letter-spacing="-1.5" fill="url(#text_grad)">Real<tspan fill="#0052FF">Browser</tspan></text>
  <text x="284" y="186" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" font-weight="500" font-size="22" letter-spacing="3" fill="#64748B" text-transform="uppercase">Local-First Multi-Identity Engine</text>
</svg>
"""

    # Write files
    os.makedirs("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src-tauri/icons", exist_ok=True)
    os.makedirs("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets", exist_ok=True)
    
    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src-tauri/icons/app-icon.svg", "w", encoding="utf-8") as f:
        f.write(svg_app_icon)

    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets/logo.svg", "w", encoding="utf-8") as f:
        f.write(svg_mark)
        
    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets/logo-light-app.svg", "w", encoding="utf-8") as f:
        f.write(svg_light_app_icon)

    with open("/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets/logo-brand.svg", "w", encoding="utf-8") as f:
        f.write(svg_brand_lockup)

    print("SVG assets refreshed cleanly!")

generate_svg_suite()
