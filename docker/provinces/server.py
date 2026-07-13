"""
MCP Server: Vietnam Provinces — Tra cứu tỉnh thành, phường xã Việt Nam
sau đợt sáp nhập 2025 (34 tỉnh/thành).

Dữ liệu lấy từ thư viện `vietnam-provinces` (PyPI), nguồn gốc từ
Tổng cục Thống kê Việt Nam, được cộng đồng duy trì và cập nhật thường
xuyên (không tự tay biên soạn dữ liệu tĩnh).
"""
import vietnam_provinces
from mcp.server.fastmcp import FastMCP
from vietnam_provinces import Province, ProvinceCode, Ward

mcp = FastMCP("VietnamProvinces", host="0.0.0.0", port=8080)


def _province_to_dict(p: Province) -> dict:
    return {
        "code": int(p.code),
        "name": p.name,
        "division_type": p.division_type.value,
        "codename": p.codename,
        "phone_code": p.phone_code,
    }


def _ward_to_dict(w: Ward) -> dict:
    return {
        "code": int(w.code),
        "name": w.name,
        "division_type": w.division_type.value,
        "codename": w.codename,
        "province_code": int(w.province_code),
    }


@mcp.tool()
def list_provinces() -> dict:
    """Liệt kê toàn bộ 34 tỉnh/thành phố trực thuộc trung ương của Việt Nam
    (theo cơ cấu hành chính sau sáp nhập 01/07/2025)."""
    provinces = [_province_to_dict(p) for p in Province.iter_all()]
    return {
        "data_version": vietnam_provinces.__data_version__,
        "total": len(provinces),
        "provinces": provinces,
    }


@mcp.tool()
def get_province_info(name: str) -> dict:
    """Tra cứu thông tin chi tiết 1 tỉnh/thành theo tên (ví dụ: 'Khánh Hòa',
    'Hà Nội', 'Cần Thơ'). Trả về mã tỉnh, mã điện thoại vùng, và số lượng
    phường/xã trực thuộc."""
    results = Province.search(name)
    if not results:
        return {"error": f"Không tìm thấy tỉnh/thành nào khớp với '{name}'"}

    province = results[0]
    ward_count = sum(1 for w in Ward.iter_all() if w.province_code == province.code)

    return {
        **_province_to_dict(province),
        "ward_count": ward_count,
        "other_matches": [p.name for p in results[1:]] if len(results) > 1 else [],
    }


@mcp.tool()
def list_wards_in_province(province_name: str) -> dict:
    """Liệt kê tất cả phường/xã thuộc 1 tỉnh/thành, theo tên tỉnh
    (ví dụ: 'Khánh Hòa'). Hữu ích để biết 1 tỉnh có bao nhiêu phường xã
    và tên gọi cụ thể."""
    matches = Province.search(province_name)
    if not matches:
        return {"error": f"Không tìm thấy tỉnh/thành nào khớp với '{province_name}'"}

    province = matches[0]
    wards = [_ward_to_dict(w) for w in Ward.iter_all() if w.province_code == province.code]

    return {
        "province": province.name,
        "ward_count": len(wards),
        "wards": wards,
    }


@mcp.tool()
def search_ward(name: str, province_name: str = "") -> dict:
    """Tìm phường/xã theo tên, có thể lọc theo tỉnh nếu muốn thu hẹp kết quả.
    Ví dụ: search_ward('phu my') hoặc search_ward('phu my', 'Hồ Chí Minh')."""
    province_code = None
    if province_name:
        matches = Province.search(province_name)
        if not matches:
            return {"error": f"Không tìm thấy tỉnh/thành nào khớp với '{province_name}'"}
        province_code = matches[0].code

    if province_code is not None:
        results = Ward.search(name, province=province_code)
    else:
        results = Ward.search(name)

    if not results:
        return {"error": f"Không tìm thấy phường/xã nào khớp với '{name}'"}

    return {
        "total": len(results),
        "wards": [_ward_to_dict(w) for w in results],
    }


@mcp.tool()
def get_legacy_province_mapping(name: str) -> dict:
    """Tra cứu 1 tỉnh CŨ (trước sáp nhập 01/07/2025) hiện đã sáp nhập
    thành tỉnh/thành nào. Ví dụ: 'Bà Rịa - Vũng Tàu' -> hiện thuộc
    'Thành phố Hồ Chí Minh'. Dùng khi người dùng hỏi về tên tỉnh cũ."""
    # Legacy search hoạt động theo code, nên trước tiên tìm theo tên hiện tại
    # trong dữ liệu legacy bằng cách quét toàn bộ tỉnh hiện tại và các
    # nguồn gốc (get_legacy_sources) của chúng.
    name_lower = name.strip().lower()
    for province in Province.iter_all():
        for legacy in province.get_legacy_sources():
            if name_lower in legacy.name.lower():
                return {
                    "legacy_province": legacy.name,
                    "merged_into": province.name,
                    "current_province_code": int(province.code),
                }
    return {"error": f"Không tìm thấy tỉnh cũ nào khớp với '{name}'"}


@mcp.tool()
def get_provinces_merged_into(name: str) -> dict:
    """Cho biết 1 tỉnh/thành HIỆN TẠI được sáp nhập từ những tỉnh cũ nào.
    Ví dụ: get_provinces_merged_into('Hồ Chí Minh') -> Bình Dương,
    Bà Rịa - Vũng Tàu, Hồ Chí Minh (cũ)."""
    matches = Province.search(name)
    if not matches:
        return {"error": f"Không tìm thấy tỉnh/thành nào khớp với '{name}'"}

    province = matches[0]
    sources = province.get_legacy_sources()
    return {
        "current_province": province.name,
        "merged_from": [s.name for s in sources],
    }


if __name__ == "__main__":
    mcp.run(transport="streamable-http")