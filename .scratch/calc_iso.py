import math
import os

def create_isometric_r_svg(scale=1.0, width=512, height=512, bg=None, stroke=False):
    """
    Constructs the isometric faceted 'R' logo.
    Grid units:
    x_step = s * sqrt(3) / 2
    y_step = s * 0.5
    """
    cx = width / 2
    cy = height / 2 - 10 # slight visual center adjustment
    
    # Unit step sizes for 30-degree isometric grid
    s = 70.0 * scale
    dx = s * math.sqrt(3) / 2  # ~60.62
    dy = s * 0.5               # 35.0
    v = s                      # 70.0 vertical step
    
    # Let's define key vertices in terms of (cx, cy)
    # Peak of the hexagon (Top center)
    p_top_peak = (cx, cy - 2.8 * v)
    
    # Outer top left
    p_out_top_left = (cx - 2 * dx, cy - 2.8 * v + 2 * dy)
    # Outer stem top (transition)
    p_out_stem_mid1 = (cx - 2 * dx, cy - 2.8 * v + 2 * dy + 1.2 * v)
    # Outer stem mid2
    p_out_stem_mid2 = (cx - 2 * dx, cy + 0.6 * v)
    # Outer stem bottom-left
    p_out_bottom_left = (cx - 2 * dx, cy + 2.2 * v)
    # Outer stem bottom-chamfer (inward)
    p_stem_bottom_inner = (cx - 1 * dx, cy + 2.2 * v - 1 * dy)
    # Stem inner bottom fold
    p_stem_inner_low = (cx - 1 * dx, cy + 0.8 * v)
    
    print("Coordinates sample:", p_top_peak, p_out_top_left)

create_isometric_r_svg()
