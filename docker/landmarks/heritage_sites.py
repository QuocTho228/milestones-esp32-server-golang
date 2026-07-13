# -*- coding: utf-8 -*-
"""
Danh sách Di tích quốc gia đặc biệt Việt Nam (đến Đợt 13, cuối 2022).
Nguồn: Wikipedia tiếng Việt - "Di tích quốc gia đặc biệt (Việt Nam)"
(https://vi.wikipedia.org/wiki/Di_tích_quốc_gia_đặc_biệt_(Việt_Nam))

LƯU Ý: Danh sách này KHÔNG đầy đủ 100% tính đến hiện tại (đã có thêm các
đợt xếp hạng mới, tổng số hiện là 153 di tích tính đến đợt 19/2026).
Dữ liệu này chỉ dùng làm khung tra cứu nhanh (tên, tỉnh, loại hình, đợt).
Để có thông tin mới nhất/chi tiết, tool sẽ kết hợp gọi Wikipedia API.
"""

HERITAGE_SITES = [
    # Đợt 1 (2009)
    {"name": "Khu trung tâm Hoàng thành Thăng Long - Hà Nội", "province": "Hà Nội", "category": "Lịch sử, khảo cổ", "batch": 1, "year": 2009, "unesco": "Di sản văn hóa thế giới (2010)"},
    {"name": "Vườn quốc gia Phong Nha - Kẻ Bàng", "province": "Quảng Bình", "category": "Lịch sử, danh lam thắng cảnh", "batch": 1, "year": 2009, "unesco": "Di sản thiên nhiên thế giới (2003)"},
    {"name": "Chiến trường Điện Biên Phủ", "province": "Điện Biên", "category": "Lịch sử", "batch": 1, "year": 2009, "unesco": None},
    {"name": "Quần thể di tích Cố đô Huế", "province": "Thừa Thiên Huế", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 1, "year": 2009, "unesco": "Di sản văn hóa thế giới (1993)"},
    {"name": "Khu lưu niệm Chủ tịch Hồ Chí Minh tại Phủ Chủ tịch", "province": "Hà Nội", "category": "Lịch sử", "batch": 1, "year": 2009, "unesco": None},
    {"name": "Dinh Độc Lập", "province": "Hồ Chí Minh", "category": "Lịch sử", "batch": 1, "year": 2009, "unesco": None},
    {"name": "Đền Hùng", "province": "Phú Thọ", "category": "Lịch sử", "batch": 1, "year": 2009, "unesco": None},
    {"name": "Đô thị cổ Hội An", "province": "Quảng Nam", "category": "Kiến trúc nghệ thuật", "batch": 1, "year": 2009, "unesco": "Di sản văn hóa thế giới (1999)"},
    {"name": "Khu đền tháp Mỹ Sơn", "province": "Quảng Nam", "category": "Kiến trúc nghệ thuật", "batch": 1, "year": 2009, "unesco": "Di sản văn hóa thế giới (1999)"},
    {"name": "Vịnh Hạ Long", "province": "Quảng Ninh", "category": "Danh lam thắng cảnh", "batch": 1, "year": 2009, "unesco": "Di sản thiên nhiên thế giới (1994)"},

    # Đợt 2 (2012)
    {"name": "Cố đô Hoa Lư", "province": "Ninh Bình", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 2, "year": 2012, "unesco": "Thuộc Quần thể danh thắng Tràng An (2014)"},
    {"name": "Khu lưu niệm Chủ tịch Tôn Đức Thắng tại Mỹ Hòa Hưng", "province": "An Giang", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Căn cứ Trung ương Cục miền Nam", "province": "Tây Ninh", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Nhà tù Côn Đảo", "province": "Bà Rịa - Vũng Tàu", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Những địa điểm Khởi nghĩa Yên Thế", "province": "Bắc Giang", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Pác Bó", "province": "Cao Bằng", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Chiến khu Tân Trào", "province": "Tuyên Quang", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Thành nhà Hồ", "province": "Thanh Hóa", "category": "Lịch sử, kiến trúc nghệ thuật, khảo cổ", "batch": 2, "year": 2012, "unesco": "Di sản văn hóa thế giới (2011)"},
    {"name": "Tràng An - Tam Cốc - Bích Động", "province": "Ninh Bình", "category": "Danh lam thắng cảnh", "batch": 2, "year": 2012, "unesco": "Thuộc Quần thể danh thắng Tràng An (2014)"},
    {"name": "Văn Miếu - Quốc Tử Giám", "province": "Hà Nội", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Khu lưu niệm Chủ tịch Hồ Chí Minh tại Kim Liên", "province": "Nghệ An", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "An toàn khu (ATK) Định Hóa", "province": "Thái Nguyên", "category": "Lịch sử", "batch": 2, "year": 2012, "unesco": None},
    {"name": "Côn Sơn - Kiếp Bạc", "province": "Hải Dương", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 2, "year": 2012, "unesco": None},

    # Đợt 3 (2012)
    {"name": "Cổ Loa", "province": "Hà Nội", "category": "Lịch sử, kiến trúc nghệ thuật, khảo cổ", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Đền Trần và Chùa Phổ Minh", "province": "Nam Định", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Bạch Đằng", "province": "Quảng Ninh", "category": "Lịch sử", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Yên Tử", "province": "Quảng Ninh", "category": "Lịch sử, danh lam thắng cảnh", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Lam Kinh", "province": "Thanh Hóa", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Khu lưu niệm Nguyễn Du", "province": "Hà Tĩnh", "category": "Lịch sử", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Chùa Keo", "province": "Thái Bình", "category": "Kiến trúc nghệ thuật", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Óc Eo - Ba Thê", "province": "An Giang", "category": "Kiến trúc nghệ thuật, khảo cổ", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Gò Tháp", "province": "Đồng Tháp", "category": "Kiến trúc nghệ thuật, khảo cổ", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Hồ Ba Bể", "province": "Bắc Kạn", "category": "Danh lam thắng cảnh", "batch": 3, "year": 2012, "unesco": None},
    {"name": "Vườn quốc gia Cát Tiên", "province": "Đồng Nai / Bình Phước / Lâm Đồng", "category": "Danh lam thắng cảnh", "batch": 3, "year": 2012, "unesco": None},

    # Đợt 4 (2013)
    {"name": "Đường Trường Sơn - Đường Hồ Chí Minh", "province": "Nhiều tỉnh miền Trung - Tây Nguyên", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Đền Hai Bà Trưng", "province": "Hà Nội", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Đền Hát Môn", "province": "Hà Nội", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Khu di tích nhà Trần tại Đông Triều", "province": "Quảng Ninh", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Rừng Trần Hưng Đạo", "province": "Cao Bằng", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Đôi bờ Hiền Lương - Bến Hải", "province": "Quảng Trị", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Thành cổ Quảng Trị", "province": "Quảng Trị", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Chiến thắng Chương Thiện", "province": "Hậu Giang", "category": "Lịch sử", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Đền Phù Đổng", "province": "Hà Nội", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Hồ Hoàn Kiếm và Đền Ngọc Sơn", "province": "Hà Nội", "category": "Lịch sử, danh lam thắng cảnh", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Đình Tây Đằng", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Chùa Bút Tháp", "province": "Bắc Ninh", "category": "Kiến trúc nghệ thuật", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Chùa Dâu", "province": "Bắc Ninh", "category": "Kiến trúc nghệ thuật", "batch": 4, "year": 2013, "unesco": None},
    {"name": "Quần đảo Cát Bà", "province": "Hải Phòng", "category": "Danh lam thắng cảnh", "batch": 4, "year": 2013, "unesco": None},

    # Đợt 5 (2014)
    {"name": "Khu lăng mộ và đền thờ các vị vua triều Lý", "province": "Bắc Ninh", "category": "Lịch sử", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Khu lăng mộ và đền thờ các vị vua triều Trần", "province": "Thái Bình", "category": "Lịch sử", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Khu đền thờ Tây Sơn Tam Kiệt", "province": "Bình Định", "category": "Lịch sử", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Địa điểm Chiến thắng Rạch Gầm - Xoài Mút", "province": "Tiền Giang", "category": "Lịch sử", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Nhà tù Sơn La", "province": "Sơn La", "category": "Lịch sử", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Nhà tù Phú Quốc", "province": "Kiên Giang", "category": "Lịch sử", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Địa đạo Vĩnh Mốc và hệ thống làng hầm Vĩnh Linh", "province": "Quảng Trị", "category": "Lịch sử", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Khu di tích Bà Triệu", "province": "Thanh Hóa", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Chùa Thầy và khu núi đá Sài Sơn, Hoàng Xá, Phượng Cách", "province": "Hà Nội", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Chùa Tây Phương", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Chùa Phật Tích", "province": "Bắc Ninh", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Đền Sóc", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Khu di tích Phố Hiến", "province": "Hưng Yên", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 5, "year": 2014, "unesco": None},
    {"name": "Cát Tiên (di tích khảo cổ)", "province": "Lâm Đồng", "category": "Khảo cổ", "batch": 5, "year": 2014, "unesco": None},

    # Đợt 6 (2015)
    {"name": "Đền thờ Nguyễn Bỉnh Khiêm", "province": "Hải Phòng", "category": "Lịch sử", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Đền Trần Thương", "province": "Hà Nam", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Tháp Bình Sơn", "province": "Vĩnh Phúc", "category": "Kiến trúc nghệ thuật", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Tây Thiên - Tam Đảo", "province": "Vĩnh Phúc", "category": "Lịch sử, danh lam thắng cảnh", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Căn cứ Bộ chỉ huy Quân giải phóng miền Nam Việt Nam", "province": "Bình Phước", "category": "Lịch sử", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Chùa Vĩnh Nghiêm", "province": "Bắc Giang", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Địa đạo Củ Chi", "province": "Hồ Chí Minh", "category": "Lịch sử", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Tháp Chăm Dương Long", "province": "Bình Định", "category": "Kiến trúc nghệ thuật", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Hang Con Moong và các di tích phụ cận", "province": "Thanh Hóa", "category": "Khảo cổ", "batch": 6, "year": 2015, "unesco": None},
    {"name": "Mộ cự thạch Hàng Gòn", "province": "Đồng Nai", "category": "Khảo cổ", "batch": 6, "year": 2015, "unesco": None},

    # Đợt 7 (2016)
    {"name": "Khởi nghĩa Bắc Sơn", "province": "Lạng Sơn", "category": "Lịch sử", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Địa điểm diễn ra Đại hội Đại biểu toàn quốc lần thứ II của Đảng", "province": "Tuyên Quang", "category": "Lịch sử", "batch": 7, "year": 2016, "unesco": None},
    {"name": "An toàn khu (ATK) Chợ Đồn", "province": "Bắc Kạn", "category": "Lịch sử", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Chùa Bổ Đà", "province": "Bắc Giang", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Quần thể An Phụ - Kính Chủ - Nhẫm Dương", "province": "Hải Dương", "category": "Lịch sử, danh lam thắng cảnh", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Chùa Keo Hành Thiện", "province": "Nam Định", "category": "Kiến trúc nghệ thuật", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Khu lưu niệm Phan Bội Châu tại Nam Đàn", "province": "Nghệ An", "category": "Lịch sử", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Phật viện Đồng Dương", "province": "Quảng Nam", "category": "Khảo cổ", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Địa điểm Chiến thắng Đăk Tô - Tân Cảnh", "province": "Kon Tum", "category": "Lịch sử", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Tháp Hòa Lai", "province": "Ninh Thuận", "category": "Kiến trúc nghệ thuật, khảo cổ", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Tháp Po Klong Garai", "province": "Ninh Thuận", "category": "Kiến trúc nghệ thuật", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Đồng Khởi Bến Tre", "province": "Bến Tre", "category": "Lịch sử", "batch": 7, "year": 2016, "unesco": None},
    {"name": "Mộ và Khu lưu niệm Nguyễn Đình Chiểu", "province": "Bến Tre", "category": "Lịch sử", "batch": 7, "year": 2016, "unesco": None},

    # Đợt 8 (2017)
    {"name": "Đền Cửa Ông", "province": "Quảng Ninh", "category": "Lịch sử", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Văn miếu Mao Điền", "province": "Hải Dương", "category": "Lịch sử", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Địa điểm khởi nghĩa Ba Tơ", "province": "Quảng Ngãi", "category": "Lịch sử", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Địa điểm Chiến thắng Biên giới năm 1950", "province": "Cao Bằng", "category": "Lịch sử", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Chùa Đọi Sơn", "province": "Hà Nam", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Đền Xưa - Chùa Giám - Đền Bia", "province": "Hải Dương", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Thành Điện Hải", "province": "Đà Nẵng", "category": "Lịch sử", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Quần thể Hương Sơn (Chùa Hương)", "province": "Hà Nội", "category": "Lịch sử, danh lam thắng cảnh", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Đình Hoành Sơn", "province": "Nghệ An", "category": "Kiến trúc nghệ thuật", "batch": 8, "year": 2017, "unesco": None},
    {"name": "Đình Chèm", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 8, "year": 2017, "unesco": None},

    # Đợt 9 (2018)
    {"name": "Tháp Nhạn", "province": "Phú Yên", "category": "Kiến trúc nghệ thuật", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Chùa Thái Lạc", "province": "Hưng Yên", "category": "Kiến trúc nghệ thuật", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Đền thờ Lê Hoàn", "province": "Thanh Hóa", "category": "Kiến trúc nghệ thuật", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Đình Tường Phiêu", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Đình Thổ Tang", "province": "Vĩnh Phúc", "category": "Kiến trúc nghệ thuật", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Đình So", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Gò Đống Đa", "province": "Hà Nội", "category": "Lịch sử", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Nhà đày Buôn Ma Thuột", "province": "Đắk Lắk", "category": "Lịch sử", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Ngũ Hành Sơn", "province": "Đà Nẵng", "category": "Danh lam thắng cảnh", "batch": 9, "year": 2018, "unesco": None},
    {"name": "Khu bảo tồn thiên nhiên Na Hang - Lâm Bình", "province": "Tuyên Quang", "category": "Danh lam thắng cảnh", "batch": 9, "year": 2018, "unesco": None},

    # Đợt 10 (2019)
    {"name": "Chi Lăng", "province": "Lạng Sơn", "category": "Lịch sử", "batch": 10, "year": 2019, "unesco": None},
    {"name": "Địa điểm Chiến thắng Xương Giang", "province": "Bắc Giang", "category": "Lịch sử", "batch": 10, "year": 2019, "unesco": None},
    {"name": "Núi Non Nước", "province": "Ninh Bình", "category": "Lịch sử, danh lam thắng cảnh", "batch": 10, "year": 2019, "unesco": None},
    {"name": "Sầm Sơn", "province": "Thanh Hóa", "category": "Lịch sử, danh lam thắng cảnh", "batch": 10, "year": 2019, "unesco": None},
    {"name": "Ruộng bậc thang Mù Cang Chải", "province": "Yên Bái", "category": "Danh lam thắng cảnh", "batch": 10, "year": 2019, "unesco": None},
    {"name": "Đình Đại Phùng", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 10, "year": 2019, "unesco": None},
    {"name": "Đền - Chùa - Đình Hai Bà Trưng", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 10, "year": 2019, "unesco": None},

    # Đợt 11 (2020)
    {"name": "Hệ thống di tích lưu niệm Chủ tịch Hồ Chí Minh tại Thừa Thiên Huế", "province": "Thừa Thiên Huế", "category": "Lịch sử", "batch": 11, "year": 2020, "unesco": None},
    {"name": "An toàn khu II Hiệp Hòa", "province": "Bắc Giang", "category": "Lịch sử", "batch": 11, "year": 2020, "unesco": None},
    {"name": "Căn cứ Cái Chanh", "province": "Bạc Liêu", "category": "Lịch sử", "batch": 11, "year": 2020, "unesco": None},
    {"name": "Đền An Xá", "province": "Hưng Yên", "category": "Kiến trúc nghệ thuật", "batch": 11, "year": 2020, "unesco": None},
    {"name": "Đình Hạ Hiệp", "province": "Hà Nội", "category": "Kiến trúc nghệ thuật", "batch": 11, "year": 2020, "unesco": None},
    {"name": "Lăng mộ và Đền thờ Nguyễn Xí", "province": "Nghệ An", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 11, "year": 2020, "unesco": None},
    {"name": "Gành Đá Đĩa", "province": "Phú Yên", "category": "Danh lam thắng cảnh", "batch": 11, "year": 2020, "unesco": None},

    # Đợt 12 (2022)
    {"name": "Quần thể di tích Tây Sơn Thượng Đạo", "province": "Gia Lai", "category": "Lịch sử", "batch": 12, "year": 2022, "unesco": None},
    {"name": "Khu lưu niệm Chủ tịch Hồ Chí Minh trên đảo Cô Tô", "province": "Quảng Ninh", "category": "Lịch sử", "batch": 12, "year": 2022, "unesco": None},
    {"name": "Khu di tích lịch sử cách mạng Việt Nam - Lào", "province": "Sơn La", "category": "Lịch sử", "batch": 12, "year": 2022, "unesco": None},
    {"name": "Thăng Long tứ trấn", "province": "Hà Nội", "category": "Lịch sử, kiến trúc nghệ thuật", "batch": 12, "year": 2022, "unesco": None},

    # Đợt 13 (2022)
    {"name": "Rộc Tưng - Gò Đá", "province": "Gia Lai", "category": "Khảo cổ", "batch": 13, "year": 2022, "unesco": None},
    {"name": "Văn hóa Sa Huỳnh", "province": "Quảng Ngãi", "category": "Khảo cổ", "batch": 13, "year": 2022, "unesco": None},
    {"name": "Cụm đình Hương Canh", "province": "Vĩnh Phúc", "category": "Kiến trúc nghệ thuật", "batch": 13, "year": 2022, "unesco": None},
    {"name": "Đền thờ Vua Mai Hắc Đế", "province": "Nghệ An", "category": "Lịch sử", "batch": 13, "year": 2022, "unesco": None},
    {"name": "Địa điểm Chiến thắng Ấp Bắc", "province": "Tiền Giang", "category": "Lịch sử", "batch": 13, "year": 2022, "unesco": None},
]