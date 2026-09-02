import ipaddress
import json
from pathlib import Path

from mmdb_writer import MMDBWriter
from netaddr import IPSet


BASE_DIR = Path(__file__).resolve().parent
ENTERPRISE_DB_FILE = BASE_DIR / "enterprise.mmdb"
ENTERPRISE_JSON_FILE = BASE_DIR / "enterprise_data.json"
REGION_CODES_JSON_FILE = BASE_DIR / "region_codes.json"
IP_INVENTORY_JSON_FILE = BASE_DIR / "ip_inventory.json"

DATABASE_TYPE = "Enterprise-Custom"
LANGUAGES = ["en", "zh-CN"]


# ==========================================
# 标准地理编码树：国家(ISO 3166-1) -> 省(GB/T 2260) -> 市(GB/T 2260) 逐层嵌套
# Standard geo-code tree: Country (ISO 3166-1) -> Province (GB/T 2260) -> City (GB/T 2260)
# 数据来源于 region_codes.json，如需增删国家/省/市，直接编辑该 JSON 文件即可。
# Data is loaded from region_codes.json; edit that file to add/remove entries.
# ==========================================
def load_region_codes(json_file):
    with open(json_file, "r", encoding="utf-8") as f:
        return json.load(f)


REGION_CODES = load_region_codes(REGION_CODES_JSON_FILE)


# ==========================================
# 企业内部 IP / 网段（键可为单 IP 或 CIDR）
# country/province/city: 对应 REGION_CODES 逐层嵌套的编码路径
# c1~c4: Branch / Supplier / Environment / Business Office
# 数据来源于 ip_inventory.json，新增/修改 IP 直接编辑该 JSON 文件即可。
# Data is loaded from ip_inventory.json; edit that file to add/update IP entries.
# ==========================================
def load_ip_inventory(json_file):
    with open(json_file, "r", encoding="utf-8") as f:
        return json.load(f)


IP_INVENTORY = load_ip_inventory(IP_INVENTORY_JSON_FILE)


def to_network(key):
    """
    单 IP 自动补全为 /32，CIDR 按原样规范化
    Normalize a single IP to a /32 network; a CIDR is normalized as-is.
    """

    return str(ipaddress.ip_network(key, strict=False))


def resolve_region(key, meta):
    """
    按 country -> province -> city 逐层校验并取值，供校验与记录构建共用，避免重复查找
    Validate and resolve country -> province -> city in one pass; shared by the
    validation step and build_record() to avoid duplicated lookups.
    """

    country = REGION_CODES.get(meta["country"])
    if country is None:
        raise ValueError(f"Unknown country for {key}: {meta['country']}")

    province = country["provinces"].get(meta["province"])
    if province is None:
        raise ValueError(f"Unknown province for {key}: {meta['province']}")

    city = province["cities"].get(meta["city"])
    if city is None:
        raise ValueError(f"Unknown city for {key}: {meta['city']}")

    return country, province, city


def validate_ip_inventory():
    """
    校验 ip_inventory.json 中每条 IP/网段格式及其地理编码是否合法
    Validate every entry in ip_inventory.json: IP/network format and geo codes.
    """

    for key, meta in IP_INVENTORY.items():
        try:
            to_network(key)
        except ValueError:
            raise ValueError(f"Invalid IP or network: {key}")

        resolve_region(key, meta)


def build_record(key, meta):
    """
    生成扁平记录：Country / Province / City 标准信息 + C1~C4 业务字段
    Build a flat record: standard Country/Province/City info + C1~C4 business fields.
    """

    country, province, city = resolve_region(key, meta)

    return {
        # 排除子层级字段(provinces/cities)，其余标准字段原样带出
        # Drop nested child fields (provinces/cities); keep the rest as-is.
        "country": {k: v for k, v in country.items() if k != "provinces"},
        "province": {"code": meta["province"], **{k: v for k, v in province.items() if k != "cities"}},
        "city": {"code": meta["city"], **city},
        "c1": meta["c1"],
        "c2": meta["c2"],
        "c3": meta["c3"],
        "c4": meta["c4"],
    }


def build_networks():
    """
    汇总生成 JSON 导出与 MMDB 写入共用的网段数据列表
    Build the list of network entries shared by both the JSON dump and MMDB write.
    """

    return [
        {
            "network": to_network(key),
            "meta": meta,
            "data": build_record(key, meta),
        }
        for key, meta in IP_INVENTORY.items()
    ]


def dump_enterprise_ip(networks, json_file):
    """
    导出企业内网 IP 清单（仅保留编码引用，不展开省市名称）
    Export the enterprise IP inventory (codes only, without expanded names).
    """

    with open(json_file, "w", encoding="utf-8") as f:
        json.dump(
            [
                {"network": item["network"], **item["meta"]}
                for item in networks
            ],
            f,
            indent=2,
            ensure_ascii=False
        )

    print(f"JSON generated: {json_file}")


def write_mmdb(networks, output_file):
    """
    将展开后的完整记录写入 MMDB 数据库文件
    Write the fully expanded records into an MMDB database file.
    """

    writer = MMDBWriter(
        ip_version=4,
        database_type=DATABASE_TYPE,
        languages=LANGUAGES,
        description={
            "en": "Enterprise internal IP database",
            "zh-CN": "企业内网 IP 数据库",
        },
        int_type="u32",
        float_type="f64",
    )

    for item in networks:
        writer.insert_network(IPSet([item["network"]]), item["data"])

    writer.to_db_file(str(output_file))

    print(f"MMDB generated: {output_file}")


def main():
    """
    生成流程：校验数据 -> 汇总网段 -> 导出 JSON -> 写入 MMDB
    Pipeline: validate data -> build network records -> dump JSON -> write MMDB.
    """

    print("===================================")
    print(" Enterprise MMDB Generator")
    print("===================================")

    validate_ip_inventory()

    networks = build_networks()

    print(f"IP count: {len(networks)}")

    for item in networks:
        meta = item["meta"]
        print(
            f"{item['network']} -> "
            f"{meta['country']}-{meta['province']}-{meta['city']} / "
            f"{meta['c1']} / "
            f"{meta['c2']} / "
            f"{meta['c3']} / "
            f"{meta['c4']}"
        )

    print()
    dump_enterprise_ip(networks, ENTERPRISE_JSON_FILE)

    print()
    print("===================================")

    write_mmdb(networks, ENTERPRISE_DB_FILE)

    print("===================================")


if __name__ == "__main__":
    main()