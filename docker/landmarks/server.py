"""
MCP Server: Vietnam Wiki-Deep & Landmarks
Tra cứu sâu Wikipedia tiếng Việt (định hướng lịch sử/địa danh) kết hợp
danh sách tĩnh Di tích quốc gia đặc biệt Việt Nam (129 di tích, đến đợt
13/2022) làm khung tra cứu nhanh theo tỉnh/loại hình.

Vì danh sách tĩnh có thể không cập nhật 100% (đã có thêm các đợt xếp hạng
mới), mọi tool đều có thể kết hợp gọi Wikipedia API để lấy thông tin mới
và chi tiết hơn.
"""
import httpx
from mcp.server.fastmcp import FastMCP

from heritage_sites import HERITAGE_SITES

mcp = FastMCP("VietnamWikiLandmarks", host="0.0.0.0", port=8080)

WIKI_API = "https://vi.wikipedia.org/w/api.php"
WIKI_SUMMARY = "https://vi.wikipedia.org/api/rest_v1/page/summary/{title}"


# ---------------------- Danh sách tĩnh: Di tích quốc gia đặc biệt ----------------------
@mcp.tool()
def search_heritage_sites(keyword: str = "", province: str = "", category: str = "") -> dict:
    """Tìm trong danh sách 129 Di tích quốc gia đặc biệt Việt Nam (đến đợt
    13/2022), lọc theo từ khóa tên, tỉnh/thành, hoặc loại hình (lịch sử,
    kiến trúc nghệ thuật, khảo cổ, danh lam thắng cảnh). Bỏ trống tham số
    nào không cần lọc. LƯU Ý: danh sách này không cập nhật 100% các đợt
    xếp hạng mới nhất; dùng get_landmark_info để tra cứu chi tiết/mới hơn."""
    keyword_l = keyword.strip().lower()
    province_l = province.strip().lower()
    category_l = category.strip().lower()

    results = []
    for site in HERITAGE_SITES:
        if keyword_l and keyword_l not in site["name"].lower():
            continue
        if province_l and province_l not in site["province"].lower():
            continue
        if category_l and category_l not in site["category"].lower():
            continue
        results.append(site)

    return {
        "total": len(results),
        "note": "Danh sách tĩnh tính đến đợt 13/2022 (129 di tích), có thể thiếu các đợt mới hơn.",
        "sites": results,
    }


@mcp.tool()
def list_heritage_sites_by_province() -> dict:
    """Thống kê số lượng Di tích quốc gia đặc biệt theo từng tỉnh/thành
    (dựa trên danh sách tĩnh đến đợt 13/2022). Hữu ích để biết tỉnh nào
    có nhiều di tích nhất."""
    counts: dict[str, int] = {}
    for site in HERITAGE_SITES:
        counts[site["province"]] = counts.get(site["province"], 0) + 1
    sorted_counts = dict(sorted(counts.items(), key=lambda x: -x[1]))
    return {"total_sites": len(HERITAGE_SITES), "by_province": sorted_counts}


# ---------------------- Wiki-deep: tra cứu sâu Wikipedia tiếng Việt ----------------------
@mcp.tool()
async def search_wikipedia(query: str, limit: int = 5) -> dict:
    """Tìm kiếm bài viết trên Wikipedia tiếng Việt theo từ khóa (nhân vật
    lịch sử, sự kiện, địa danh, khái niệm...). Trả về danh sách tiêu đề
    và đoạn trích ngắn để chọn bài phù hợp trước khi gọi get_wiki_summary."""
    async with httpx.AsyncClient(timeout=10) as client:
        resp = await client.get(
            WIKI_API,
            params={
                "action": "query",
                "list": "search",
                "srsearch": query,
                "srlimit": limit,
                "format": "json",
            },
        )
        data = resp.json()

    hits = data.get("query", {}).get("search", [])
    if not hits:
        return {"error": f"Không tìm thấy bài viết nào khớp với '{query}'"}

    return {
        "total": len(hits),
        "results": [
            {"title": h["title"], "snippet": h["snippet"].replace('<span class="searchmatch">', "").replace("</span>", "")}
            for h in hits
        ],
    }


@mcp.tool()
async def get_wiki_summary(title: str) -> dict:
    """Lấy tóm tắt (đoạn mở đầu) của 1 bài Wikipedia tiếng Việt theo TÊN
    BÀI CHÍNH XÁC (dùng search_wikipedia trước nếu chưa chắc tên bài).
    Phù hợp cho nhân vật lịch sử, sự kiện, địa danh, khái niệm văn hóa."""
    async with httpx.AsyncClient(timeout=10) as client:
        resp = await client.get(WIKI_SUMMARY.format(title=title.replace(" ", "_")))
        if resp.status_code == 404:
            return {"error": f"Không tìm thấy bài viết '{title}' trên Wikipedia tiếng Việt"}
        data = resp.json()

    return {
        "title": data.get("title"),
        "extract": data.get("extract"),
        "url": data.get("content_urls", {}).get("desktop", {}).get("page"),
    }


@mcp.tool()
async def get_landmark_info(name: str) -> dict:
    """Tra cứu thông tin 1 địa danh/di tích Việt Nam theo tên: kết hợp
    kiểm tra xem có trong danh sách Di tích quốc gia đặc biệt hay không,
    và lấy tóm tắt trực tiếp từ Wikipedia tiếng Việt để có mô tả chi tiết,
    cập nhật. Đây là tool nên dùng đầu tiên khi được hỏi về 1 địa danh cụ
    thể (ví dụ: 'Chùa Một Cột', 'Vịnh Hạ Long', 'Địa đạo Củ Chi')."""
    name_l = name.strip().lower()
    heritage_match = next((s for s in HERITAGE_SITES if name_l in s["name"].lower()), None)

    async with httpx.AsyncClient(timeout=10) as client:
        resp = await client.get(WIKI_SUMMARY.format(title=name.strip().replace(" ", "_")))
        wiki_data = resp.json() if resp.status_code == 200 else None

    if wiki_data is None and heritage_match is None:
        return {"error": f"Không tìm thấy thông tin cho '{name}'. Hãy thử search_wikipedia để tìm tên bài chính xác."}

    result = {"query": name}
    if heritage_match:
        result["heritage_status"] = heritage_match
    if wiki_data:
        result["wikipedia"] = {
            "title": wiki_data.get("title"),
            "extract": wiki_data.get("extract"),
            "url": wiki_data.get("content_urls", {}).get("desktop", {}).get("page"),
        }
    return result


if __name__ == "__main__":
    mcp.run(transport="streamable-http")