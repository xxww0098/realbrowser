import math
import os

def pt_str(pts):
    return " ".join(f"{round(p[0], 2)},{round(p[1], 2)}" for p in pts)

def build_svg_content():
    # Let's define the precise 3D faceted ribbon R model
    # Grid: 512 x 512
    # All hex-isometric angles are at 30° / 60° / 90°
    
    # 1. Coordinate Definitions:
    # Outer Hexagon Vertices
    V_TOP_PEAK   = (256.0, 56.0)    # Top Peak
    V_TOP_LEFT   = (108.0, 141.5)   # Upper Left Outer Corner (30 deg slope from peak)
    V_TOP_RIGHT  = (404.0, 141.5)   # Upper Right Outer Corner
    
    V_MID_RIGHT  = (404.0, 276.0)   # Lower Outer Corner of Bowl
    V_BOT_LEFT   = (108.0, 396.0)   # Bottom Left Outer Corner
    V_BOT_CHAMF  = (176.0, 356.7)   # Bottom Left Inner Chamfer (30 deg up-right from V_BOT_LEFT)
    
    # Stem Inner Boundary
    V_STEM_IN_B  = (176.0, 356.7)
    V_STEM_IN_M  = (176.0, 230.0)
    V_STEM_IN_T  = (176.0, 180.8)   # 30 deg slope from V_HOLE_TOP
    
    # Inner Counter (Hole of the R)
    V_HOLE_TOP   = (256.0, 134.6)   # Inner Peak (parallel to outer top peak)
    V_HOLE_TL    = (176.0, 180.8)
    V_HOLE_TR    = (336.0, 180.8)
    V_HOLE_BR    = (336.0, 240.0)
    V_HOLE_BL    = (176.0, 240.0)
    
    # Bowl Inner Fold / Waist
    V_WAIST_TOP  = (256.0, 240.0)
    V_WAIST_MID  = (256.0, 286.0)
    V_BOWL_IN_R  = (336.0, 286.0)
    
    # The Leg (Folded Origami Ribbon extending down-right)
    # Upper facet of the leg
    V_LEG_TOP_R  = (404.0, 371.4)   # Outer top tip of the leg
    V_LEG_END_B  = (404.0, 412.0)   # Outer bottom tip of the leg
    V_LEG_FOLD_M = (324.0, 325.2)   # Central ridge fold of the leg
    V_LEG_FOLD_B = (324.0, 372.0)   # Lower fold point
    V_LEG_IN_BOT = (244.0, 418.2)   # Bottom inner point of the leg ribbon
    V_LEG_STEM_J = (176.0, 286.0)   # Stem junction
    
    # Roof facet splits:
    V_ROOF_L_SPLIT = (182.0, 98.8)
    V_ROOF_R_SPLIT = (330.0, 98.8)
    
    # Let's define the facets
    facets = [
        # Facet 1: Left Stem Top Chamfer / Facet
        {
            "id": "stem_top",
            "pts": [V_TOP_LEFT, V_ROOF_L_SPLIT, V_HOLE_TL, (108.0, 180.8)],
            "fill": "url(#grad_stem_top)",
            "stroke": "#1E40AF",
            "opacity": 0.95
        },
        # Facet 2: Left Stem Main Center Bar
        {
            "id": "stem_main",
            "pts": [(108.0, 180.8), V_HOLE_TL, V_STEM_IN_M, (108.0, 300.0)],
            "fill": "url(#grad_stem_main)",
            "stroke": "#1E40AF",
            "opacity": 1.0
        },
        # Facet 3: Left Stem Bottom Body & Chamfer
        {
            "id": "stem_bottom",
            "pts": [(108.0, 300.0), V_STEM_IN_M, V_BOT_CHAMF, V_BOT_LEFT],
            "fill": "url(#grad_stem_bot)",
            "stroke": "#1E40AF",
            "opacity": 1.0
        },
        # Facet 4: Top Roof Left Slope
        {
            "id": "roof_left",
            "pts": [V_TOP_LEFT, V_TOP_PEAK, V_HOLE_TOP, V_ROOF_L_SPLIT],
            "fill": "url(#grad_roof_left)",
            "stroke": "#1E40AF",
            "opacity": 0.95
        },
        # Facet 5: Top Roof Peak & Right Slope (Highlight Face)
        {
            "id": "roof_peak_highlight",
            "pts": [V_TOP_PEAK, V_TOP_RIGHT, V_HOLE_TR, V_HOLE_TOP],
            "fill": "url(#grad_roof_peak)",
            "stroke": "#1E40AF",
            "opacity": 1.0
        },
        # Facet 6: Upper Right Bowl - Outer Vertical Panel
        {
            "id": "bowl_outer",
            "pts": [V_TOP_RIGHT, V_MID_RIGHT, V_BOWL_IN_R, V_HOLE_TR],
            "fill": "url(#grad_bowl_outer)",
            "stroke": "#1E40AF",
            "opacity": 1.0
        },
        # Facet 7: Upper Right Bowl - Bottom Return to Waist
        {
            "id": "bowl_return",
            "pts": [V_MID_RIGHT, (364.0, 299.0), V_WAIST_MID, V_BOWL_IN_R],
            "fill": "url(#grad_bowl_return)",
            "stroke": "#1E40AF",
            "opacity": 0.95
        },
        # Facet 8: Central Waist / Horizontal Connector
        {
            "id": "waist_bar",
            "pts": [V_HOLE_BL, V_HOLE_BR, V_WAIST_MID, V_LEG_STEM_J],
            "fill": "url(#grad_waist)",
            "stroke": "#1E40AF",
            "opacity": 1.0
        },
        # Facet 9: Leg - Top Light Facet (Upper Origami Surface)
        {
            "id": "leg_upper",
            "pts": [V_WAIST_MID, (364.0, 299.0), V_LEG_TOP_R, V_LEG_FOLD_M],
            "fill": "url(#grad_leg_upper)",
            "stroke": "#1E40AF",
            "opacity": 1.0
        },
        # Facet 10: Leg - Outer Tip Chamfer
        {
            "id": "leg_tip",
            "pts": [V_LEG_TOP_R, V_LEG_END_B, V_LEG_FOLD_B, V_LEG_FOLD_M],
            "fill": "url(#grad_leg_tip)",
            "stroke": "#1E40AF",
            "opacity": 0.95
        },
        # Facet 11: Leg - Underfold Shadow / 3D Flange
        {
            "id": "leg_shadow",
            "pts": [V_LEG_STEM_J, V_WAIST_MID, V_LEG_FOLD_M, V_LEG_FOLD_B, V_LEG_IN_BOT],
            "fill": "url(#grad_leg_shadow)",
            "stroke": "#1E40AF",
            "opacity": 1.0
        }
    ]
    
    return facets

print("SVG facet model constructed")
