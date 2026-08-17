import math
import os

def create_svg_faceted_r(
    width=512, 
    height=512, 
    style='standard', # 'standard', 'vibrant', 'dark_card', 'light_card', 'monochrome'
    with_bg=False,
    bg_radius=112
):
    """
    Generate the SVG string for the faceted R logo.
    """
    
    # 512x512 coordinate box centered around (256, 240)
    # Hexagonal geometry parameters:
    # Outer radius R_hex = 195
    # Inner thickness = 68
    
    # Let's define key vertices:
    # Top Peak
    T_PEAK = (256, 52)
    
    # Roof mid-top highlight vertices
    T_ROOF_L = (196, 88)
    T_ROOF_R = (316, 88)
    
    # Outer Top-Left and Top-Right corners
    TL_OUT = (96, 144)
    TR_OUT = (416, 144)
    
    # Left Stem outer vertical edge
    SL_TOP = (96, 144)
    SL_MID = (96, 260)
    SL_BOT = (96, 404)
    SL_TIP = (168, 362)  # Chamfer bottom-left up-right at 30 deg
    
    # Left Stem inner vertical edge
    SI_BOT = (168, 362)
    SI_MID = (168, 252)
    SI_TOP = (168, 186)
    
    # Inner Counter (Hole of the R)
    H_TOP = (256, 136)
    H_TL  = (168, 186)
    H_TR  = (344, 186)
    H_BR  = (344, 252)
    H_MID = (256, 252)
    H_BL  = (168, 252)
    
    # Upper Right Bowl outer edges
    R_TOP = (416, 144)
    R_MID = (416, 276)
    R_BOT = (344, 318)
    
    # Waist / Central fold
    W_CEN = (256, 252)
    W_MID = (256, 318)
    
    # Leg (Folded 3D ribbon extending down-right)
    # Upper facet of leg
    LEG_TOP_START = (256, 252)
    LEG_TOP_OUT = (344, 318)
    LEG_TIP_TOP = (416, 360)
    LEG_TIP_END = (416, 404)
    
    # Lower facet / underside shadow of leg
    LEG_BOT_INNER = (256, 318)
    LEG_BOT_FOLD  = (344, 384)
    LEG_BOT_TIP   = (344, 446)
    
    # Let's define the polygons for each facet:
    facets = [
        # 1. Left stem top corner (upper left triangle/quad)
        {
            "id": "stem_top_corner",
            "points": [TL_OUT, T_ROOF_L, SI_TOP, (96, 186)],
            "fill": "url(#grad_stem_top)",
            "base_color": "#2563EB"
        },
        # 2. Left stem main vertical body
        {
            "id": "stem_main",
            "points": [(96, 186), SI_TOP, SI_BOT, SL_BOT],
            "fill": "url(#grad_stem_main)",
            "base_color": "#0066FF"
        },
        # 3. Stem bottom chamfer (angled cut)
        {
            "id": "stem_bottom_chamfer",
            "points": [SL_BOT, SL_TIP, (168, 330), (96, 372)],
            "fill": "url(#grad_stem_bot)",
            "base_color": "#0052D9"
        },
        # 4. Top Roof - Left Slanted Section
        {
            "id": "roof_left",
            "points": [TL_OUT, T_PEAK, H_TOP, SI_TOP],
            "fill": "url(#grad_roof_left)",
            "base_color": "#3B82F6"
        },
        # 5. Top Roof - Peak Highlight Diamond / Triangle
        {
            "id": "roof_peak_highlight",
            "points": [T_PEAK, TR_OUT, H_TR, H_TOP],
            "fill": "url(#grad_roof_peak)",
            "base_color": "#60A5FA"
        },
        # 6. Upper Right Bowl - Outer Vertical Panel
        {
            "id": "bowl_outer_right",
            "points": [TR_OUT, R_MID, H_BR, H_TR],
            "fill": "url(#grad_bowl_right)",
            "base_color": "#2563EB"
        },
        # 7. Upper Right Bowl - Inward Lower Curve / Fold
        {
            "id": "bowl_bottom_fold",
            "points": [R_MID, R_BOT, W_MID, H_BR],
            "fill": "url(#grad_bowl_bottom)",
            "base_color": "#1D4ED8"
        },
        # 8. Central Waist / Bridge Facet
        {
            "id": "waist_bridge",
            "points": [H_BL, H_BR, W_MID, (168, 318)],
            "fill": "url(#grad_waist)",
            "base_color": "#1E40AF"
        },
        # 9. Lower Right Leg - Upper Facet (Catching top light)
        {
            "id": "leg_top_facet",
            "points": [W_MID, R_BOT, LEG_TIP_TOP, LEG_BOT_FOLD],
            "fill": "url(#grad_leg_top)",
            "base_color": "#38BDF8"
        },
        # 10. Lower Right Leg - Front Face / Fold Panel
        {
            "id": "leg_front_facet",
            "points": [LEG_BOT_FOLD, LEG_TIP_TOP, LEG_TIP_END, (344, 404)],
            "fill": "url(#grad_leg_front)",
            "base_color": "#0284C7"
        },
        # 11. Lower Right Leg - Underside Shadow Facet
        {
            "id": "leg_bottom_shadow",
            "points": [(168, 318), W_MID, LEG_BOT_FOLD, (256, 384)],
            "fill": "url(#grad_leg_shadow)",
            "base_color": "#0F3BA0"
        },
        # 12. Lower Right Leg - Bottom Diagonal Extension / Tail
        {
            "id": "leg_tail_facet",
            "points": [(256, 384), LEG_BOT_FOLD, (344, 404), (256, 446)],
            "fill": "url(#grad_leg_tail)",
            "base_color": "#0250D6"
        }
    ]
    
    return facets

print("Facet model defined")
