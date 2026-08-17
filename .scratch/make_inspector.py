import base64
import os

# Read original PNG as base64
with open('/Users/xxww/.gemini/antigravity/brain/da4532d0-acb7-4cb8-927a-099242128d6e/.user_uploaded/media_1786769906670.png', 'rb') as f:
    orig_b64 = base64.b64encode(f.read()).decode('utf-8')

# Read generated SVG
with open('/Volumes/Acasis/Code/REPO/realbrowser/apps/desktop/src/assets/logo-white.svg', 'r', encoding='utf-8') as f:
    svg_content = f.read()

html = f"""<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>1:1 SVG 复刻精确对比验证</title>
  <style>
    body {{
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: #F8FAFC;
      color: #0F172A;
      margin: 0;
      padding: 30px;
    }}
    .header {{
      max-width: 1000px;
      margin: 0 auto 30px;
    }}
    .title {{
      font-size: 24px;
      font-weight: bold;
    }}
    .subtitle {{
      color: #64748B;
      font-size: 14px;
      margin-top: 4px;
    }}
    .comparison-grid {{
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 24px;
      max-width: 1000px;
      margin: 0 auto 40px;
    }}
    .card {{
      background: #FFFFFF;
      border: 1px solid #E2E8F0;
      border-radius: 16px;
      padding: 24px;
      box-shadow: 0 4px 12px rgba(0,0,0,0.04);
      display: flex;
      flex-direction: column;
      align-items: center;
    }}
    .card h3 {{
      margin: 0 0 16px;
      font-size: 16px;
      color: #334155;
    }}
    .img-box {{
      width: 280px;
      height: 280px;
      display: flex;
      align-items: center;
      justify-content: center;
      border: 1px dashed #CBD5E1;
      border-radius: 12px;
      background: #FFFFFF;
      overflow: hidden;
    }}
    .img-box img, .img-box svg {{
      width: 85%;
      height: 85%;
      object-fit: contain;
    }}
    .sizes-row {{
      display: flex;
      gap: 24px;
      align-items: center;
      justify-content: center;
      margin-top: 30px;
      padding-top: 20px;
      border-top: 1px solid #E2E8F0;
      width: 100%;
    }}
    .size-box {{
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 8px;
      font-size: 12px;
      color: #64748B;
    }}
  </style>
</head>
<body>
  <div class="header">
    <div class="title">RealBrowser 1:1 SVG 复刻对比</div>
    <div class="subtitle">线条干净平直 · 渐变多面光泽 · 纯白背景 · 像素级几何拟合</div>
  </div>

  <div class="comparison-grid">
    <!-- Original Reference -->
    <div class="card">
      <h3>原图参考（Original PNG）</h3>
      <div class="img-box">
        <img src="data:image/png;base64,{orig_b64}" alt="Original" />
      </div>
    </div>

    <!-- 1:1 SVG Recreation -->
    <div class="card">
      <h3>1:1 SVG 矢量复刻（Vector SVG）</h3>
      <div class="img-box">
        {svg_content}
      </div>
    </div>
  </div>

  <div class="card" style="max-width: 1000px; margin: 0 auto;">
    <h3>多尺寸缩放渲染清晰度 (Multi-scale Preview)</h3>
    <div class="sizes-row">
      <div class="size-box">
        <div style="width: 128px; height: 128px; background: white; border: 1px solid #E2E8F0; border-radius: 8px; display: flex; align-items: center; justify-content: center;">
          {svg_content.replace('width="100%" height="100%"', 'width="110" height="110"')}
        </div>
        <span>128px</span>
      </div>
      <div class="size-box">
        <div style="width: 64px; height: 64px; background: white; border: 1px solid #E2E8F0; border-radius: 6px; display: flex; align-items: center; justify-content: center;">
          {svg_content.replace('width="100%" height="100%"', 'width="56" height="56"')}
        </div>
        <span>64px</span>
      </div>
      <div class="size-box">
        <div style="width: 32px; height: 32px; background: white; border: 1px solid #E2E8F0; border-radius: 4px; display: flex; align-items: center; justify-content: center;">
          {svg_content.replace('width="100%" height="100%"', 'width="28" height="28"')}
        </div>
        <span>32px</span>
      </div>
      <div class="size-box">
        <div style="width: 24px; height: 24px; background: white; border: 1px solid #E2E8F0; border-radius: 4px; display: flex; align-items: center; justify-content: center;">
          {svg_content.replace('width="100%" height="100%"', 'width="20" height="20"')}
        </div>
        <span>24px</span>
      </div>
      <div class="size-box">
        <div style="width: 16px; height: 16px; background: white; border: 1px solid #E2E8F0; border-radius: 3px; display: flex; align-items: center; justify-content: center;">
          {svg_content.replace('width="100%" height="100%"', 'width="14" height="14"')}
        </div>
        <span>16px</span>
      </div>
    </div>
  </div>
</body>
</html>
"""

with open("/Volumes/Acasis/Code/REPO/realbrowser/.scratch/inspect_1to1.html", "w", encoding="utf-8") as f:
    f.write(html)

print("Inspection tool created at .scratch/inspect_1to1.html")
