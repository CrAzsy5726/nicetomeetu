# -*- coding: utf-8 -*-
import numpy as np
from PIL import Image
from scipy import ndimage

FUZZ = 30      # 到背景色的欧氏距离阈值
FEATHER = 2.0  # 羽化强度（高斯模糊 sigma）


def remove_bg(path):
    img = Image.open(path).convert('RGBA')
    arr = np.array(img).astype(np.int16)  # H, W, 4
    h, w = arr.shape[:2]
    rgb = arr[:, :, :3]

    # 背景色：四角平均
    corners = np.array([rgb[0, 0], rgb[0, w - 1], rgb[h - 1, 0], rgb[h - 1, w - 1]])
    bg = corners.mean(axis=0)

    # 到背景色的距离
    dist = np.sqrt(((rgb - bg) ** 2).sum(axis=2))
    is_bg = dist < FUZZ

    # 连通域
    labeled, n = ndimage.label(is_bg)

    # 边缘触碰的标签
    edge = set(labeled[0, :].tolist()) | set(labeled[-1, :].tolist()) \
        | set(labeled[:, 0].tolist()) | set(labeled[:, -1].tolist())
    edge.discard(0)
    edge_bg = np.isin(labeled, list(edge))

    # 前景掩码，轻微腐蚀去掉反锯齿白边，再羽化
    fg = (~edge_bg).astype(np.uint8)
    fg = ndimage.binary_erosion(fg, iterations=1).astype(np.float32)
    fg_blur = ndimage.gaussian_filter(fg, sigma=FEATHER)

    alpha = (np.clip(fg_blur, 0, 1) * 255).astype(np.uint8)
    out_rgb = arr[:, :, :3].astype(np.uint8)
    result = Image.fromarray(np.dstack([out_rgb, alpha]), 'RGBA')
    result.save(path)


for name in ['xiaoman', 'aye', 'zhou']:
    p = f'public/characters/{name}.png'
    remove_bg(p)
    print('done', name)
