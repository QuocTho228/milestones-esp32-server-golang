"""
MCP Server: Vietnamese Proverbs — Ca dao, tục ngữ Việt Nam
106 câu phổ biến, tự biên soạn kèm giải nghĩa và nhãn chủ đề, phục vụ
mục đích giáo dục (học Ngữ văn, tìm hiểu văn hóa dân gian).
"""
import random

from mcp.server.fastmcp import FastMCP

from proverbs_data import PROVERBS

mcp = FastMCP("VietnameseProverbs", host="0.0.0.0", port=8080)


@mcp.tool()
def list_themes() -> dict:
    """Liệt kê tất cả chủ đề (ví dụ: biết ơn, hiếu thảo, chăm chỉ, kiên
    trì, đoàn kết...) hiện có trong kho ca dao tục ngữ, để biết có thể
    tìm theo những chủ đề nào."""
    themes: dict[str, int] = {}
    for p in PROVERBS:
        for t in p["themes"]:
            themes[t] = themes.get(t, 0) + 1
    return {"total_themes": len(themes), "themes": dict(sorted(themes.items()))}


@mcp.tool()
def search_by_theme(theme: str, random_pick: bool = False) -> dict:
    """Tìm ca dao/tục ngữ theo chủ đề (ví dụ: 'biết ơn', 'hiếu thảo',
    'chăm chỉ', 'kiên trì', 'đoàn kết', 'trung thực', 'tiết kiệm', 'học
    tập', 'khiêm tốn', 'tình bạn', 'tình người', 'thận trọng', 'nhân
    quả', 'gia đình', 'thời gian', 'quê hương', 'kinh nghiệm dân gian',
    'ứng xử', 'đạo đức'). Đặt random_pick=True để chỉ lấy ngẫu nhiên 1
    câu phù hợp (hữu ích khi người dùng muốn '1 câu' cụ thể)."""
    theme_l = theme.strip().lower()
    matches = [p for p in PROVERBS if any(theme_l in t for t in p["themes"])]

    if not matches:
        return {"error": f"Không tìm thấy câu nào thuộc chủ đề '{theme}'. Dùng list_themes để xem các chủ đề có sẵn."}

    if random_pick:
        pick = random.choice(matches)
        return {"theme": theme, "proverb": pick}

    return {"theme": theme, "total": len(matches), "proverbs": matches}


@mcp.tool()
def search_by_keyword(keyword: str) -> dict:
    """Tìm ca dao/tục ngữ theo từ khóa xuất hiện trong câu hoặc trong
    phần giải nghĩa (ví dụ: 'mẹ', 'cha', 'học', 'bạn')."""
    keyword_l = keyword.strip().lower()
    matches = [
        p for p in PROVERBS
        if keyword_l in p["text"].lower() or keyword_l in p["meaning"].lower()
    ]
    if not matches:
        return {"error": f"Không tìm thấy câu nào chứa từ khóa '{keyword}'"}
    return {"total": len(matches), "proverbs": matches}


@mcp.tool()
def random_proverb() -> dict:
    """Lấy ngẫu nhiên 1 câu ca dao/tục ngữ bất kỳ trong kho, kèm giải
    nghĩa. Dùng khi người dùng muốn 'một câu' mà không chỉ định chủ đề."""
    return {"proverb": random.choice(PROVERBS)}


@mcp.tool()
def explain_proverb(text: str) -> dict:
    """Tra cứu giải nghĩa của 1 câu ca dao/tục ngữ cụ thể theo văn bản
    (không cần khớp 100%, chỉ cần chứa cụm từ chính)."""
    text_l = text.strip().lower()
    match = next((p for p in PROVERBS if text_l in p["text"].lower()), None)
    if not match:
        return {"error": f"Không tìm thấy câu nào khớp với '{text}' trong kho 106 câu hiện có."}
    return match


if __name__ == "__main__":
    mcp.run(transport="streamable-http")