"""
MCP Server: Utilities — Thời tiết, Giờ giấc, Máy tính, Đổi đơn vị.
Gộp chung 1 server cho gọn, vì đều là các tool nhỏ, không cần API key.
"""
import ast
import operator
from datetime import datetime
from zoneinfo import ZoneInfo

import httpx
from mcp.server.fastmcp import FastMCP

mcp = FastMCP("Utilities", host="0.0.0.0", port=8080)


# ---------------------- Thời tiết (Open-Meteo, miễn phí, toàn cầu) ----------------------
@mcp.tool()
async def get_weather(city: str) -> dict:
    """Lấy thời tiết hiện tại theo tên thành phố (hỗ trợ mọi thành phố trên thế giới)."""
    async with httpx.AsyncClient(timeout=10) as client:
        geo = await client.get(
            "https://geocoding-api.open-meteo.com/v1/search",
            params={"name": city, "count": 1, "language": "vi"},
        )
        results = geo.json().get("results")
        if not results:
            return {"error": f"Không tìm thấy thành phố '{city}'"}
        loc = results[0]

        w = await client.get(
            "https://api.open-meteo.com/v1/forecast",
            params={
                "latitude": loc["latitude"],
                "longitude": loc["longitude"],
                "current": "temperature_2m,relative_humidity_2m,wind_speed_10m,weather_code",
                "timezone": "auto",
            },
        )
        cur = w.json()["current"]

    return {
        "city": loc["name"],
        "country": loc.get("country", ""),
        "temperature_c": cur["temperature_2m"],
        "humidity_percent": cur["relative_humidity_2m"],
        "wind_speed_kmh": cur["wind_speed_10m"],
    }


# ---------------------- Giờ giấc / múi giờ ----------------------
@mcp.tool()
def get_current_time(timezone: str = "Asia/Ho_Chi_Minh") -> dict:
    """Lấy ngày giờ hiện tại theo múi giờ (mặc định giờ Việt Nam).
    Ví dụ timezone: 'Asia/Ho_Chi_Minh', 'Asia/Tokyo', 'America/New_York', 'Europe/London'."""
    try:
        now = datetime.now(ZoneInfo(timezone))
    except Exception:
        return {"error": f"Múi giờ '{timezone}' không hợp lệ"}
    return {
        "timezone": timezone,
        "datetime": now.strftime("%H:%M:%S %d/%m/%Y"),
        "weekday": now.strftime("%A"),
    }


# ---------------------- Máy tính (phục vụ học tập) ----------------------
_OPS = {
    ast.Add: operator.add,
    ast.Sub: operator.sub,
    ast.Mult: operator.mul,
    ast.Div: operator.truediv,
    ast.Pow: operator.pow,
    ast.Mod: operator.mod,
    ast.USub: operator.neg,
}


def _eval_node(node):
    if isinstance(node, ast.Constant) and isinstance(node.value, (int, float)):
        return node.value
    if isinstance(node, ast.BinOp) and type(node.op) in _OPS:
        return _OPS[type(node.op)](_eval_node(node.left), _eval_node(node.right))
    if isinstance(node, ast.UnaryOp) and type(node.op) in _OPS:
        return _OPS[type(node.op)](_eval_node(node.operand))
    raise ValueError("Biểu thức không hợp lệ hoặc chứa ký tự không được phép")


@mcp.tool()
def calculate(expression: str) -> dict:
    """Tính toán biểu thức số học, ví dụ: '12 * (3 + 4)', '2 ** 10', '100 / 3'."""
    try:
        result = _eval_node(ast.parse(expression, mode="eval").body)
        return {"expression": expression, "result": result}
    except Exception as e:
        return {"error": str(e)}


# ---------------------- Đổi đơn vị (phục vụ học tập: lý/hóa/toán) ----------------------
_UNIT_TABLE = {
    ("km", "m"): 1000, ("m", "km"): 0.001,
    ("m", "cm"): 100, ("cm", "m"): 0.01,
    ("kg", "g"): 1000, ("g", "kg"): 0.001,
    ("mile", "km"): 1.60934, ("km", "mile"): 1 / 1.60934,
    ("lb", "kg"): 0.453592, ("kg", "lb"): 1 / 0.453592,
    ("l", "ml"): 1000, ("ml", "l"): 0.001,
}


@mcp.tool()
def convert_unit(value: float, from_unit: str, to_unit: str) -> dict:
    """Đổi đơn vị đo lường: độ dài (km/m/cm/mile), khối lượng (kg/g/lb),
    thể tích (l/ml), nhiệt độ (celsius/fahrenheit)."""
    from_unit, to_unit = from_unit.lower(), to_unit.lower()

    if from_unit == "celsius" and to_unit == "fahrenheit":
        return {"result": round(value * 9 / 5 + 32, 4)}
    if from_unit == "fahrenheit" and to_unit == "celsius":
        return {"result": round((value - 32) * 5 / 9, 4)}

    factor = _UNIT_TABLE.get((from_unit, to_unit))
    if factor is None:
        return {"error": f"Không hỗ trợ đổi từ '{from_unit}' sang '{to_unit}'"}
    return {"result": round(value * factor, 6)}


if __name__ == "__main__":
    mcp.run(transport="streamable-http")