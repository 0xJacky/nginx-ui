import os
import time
import requests

BASE_URL = "https://geo.datav.aliyun.com/areas_v3/bound"
SAVE_DIR = "maps"

os.makedirs(SAVE_DIR, exist_ok=True)

# 需要下载的行政区代码
AREA_CODES = [
    "100000",  # 全国

    "110000",  # 北京市
    "120000",  # 天津市
    "130000",  # 河北省
    "140000",  # 山西省
    "150000",  # 内蒙古自治区

    "210000",  # 辽宁省
    "220000",  # 吉林省
    "230000",  # 黑龙江省

    "310000",  # 上海市
    "320000",  # 江苏省
    "330000",  # 浙江省
    "340000",  # 安徽省
    "350000",  # 福建省
    "360000",  # 江西省
    "370000",  # 山东省

    "410000",  # 河南省
    "420000",  # 湖北省
    "430000",  # 湖南省
    "440000",  # 广东省
    "450000",  # 广西壮族自治区
    "460000",  # 海南省

    "500000",  # 重庆市

    "510000",  # 四川省
    "520000",  # 贵州省
    "530000",  # 云南省
    "540000",  # 西藏自治区

    "610000",  # 陕西省
    "620000",  # 甘肃省
    "630000",  # 青海省
    "640000",  # 宁夏回族自治区
    "650000",  # 新疆维吾尔自治区

    "710000",  # 台湾省
    "810000",  # 香港特别行政区
    "820000",  # 澳门特别行政区
]

headers = {
    "User-Agent": "Mozilla/5.0",
}

for code in AREA_CODES:
    url = f"{BASE_URL}/{code}_full.json"

    try:
        print(f"Downloading {url}")

        resp = requests.get(
            url,
            headers=headers,
            timeout=30,
        )

        if resp.status_code == 200:
            path = os.path.join(
                SAVE_DIR,
                f"{code}_full.json"
            )

            with open(path, "wb") as f:
                f.write(resp.content)

            print(f"✓ Saved: {path}")

        else:
            print(
                f"✗ Failed {code}, "
                f"status={resp.status_code}"
            )

    except Exception as e:
        print(f"✗ Error {code}: {e}")

    time.sleep(0.5)

print("Done")