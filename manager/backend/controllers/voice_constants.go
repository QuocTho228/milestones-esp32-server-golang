package controllers

import "strings"

// VoiceOption là tùy chọn giọng đọc
type VoiceOption struct {
	Value string `json:"value"` // Giá trị giọng đọc
	Label string `json:"label"` // Tên hiển thị giọng đọc
}

// VoiceOptions định nghĩa danh sách giọng đọc cho từng provider
var VoiceOptions = map[string][]VoiceOption{
	// Danh sách giọng Edge TTS (Tiếng Việt)
	// Tham khảo: https://learn.microsoft.com/en-us/azure/ai-services/speech-service/language-support
	"edge": {
		// Tiếng Việt
		{Value: "vi-VN-HoaiMyNeural", Label: "Hoài My (Nữ)"},
		{Value: "vi-VN-NamMinhNeural", Label: "Nam Minh (Nam)"},
		// Tiếng Anh
		{Value: "en-US-AriaNeural", Label: "Aria - Tiếng Anh Mỹ (Nữ)"},
		{Value: "en-US-GuyNeural", Label: "Guy - Tiếng Anh Mỹ (Nam)"},
		{Value: "en-US-JennyNeural", Label: "Jenny - Tiếng Anh Mỹ (Nữ)"},
		{Value: "en-GB-SoniaNeural", Label: "Sonia - Tiếng Anh Anh (Nữ)"},
		{Value: "en-GB-RyanNeural", Label: "Ryan - Tiếng Anh Anh (Nam)"},
		// Tiếng Trung
		{Value: "zh-CN-XiaoxiaoNeural", Label: "Tiểu Tiểu (nữ)"},
		{Value: "zh-CN-YunxiNeural", Label: "Vân Hi (nam)"},
	},

	// Danh sách giọng Microsoft TTS (Tiếng Việt)
	"microsoft": {
		// Tiếng Việt
		{Value: "vi-VN-HoaiMyNeural", Label: "Hoài My (Nữ)"},
		{Value: "vi-VN-NamMinhNeural", Label: "Nam Minh (Nam)"},
		// Tiếng Anh
		{Value: "en-US-AriaNeural", Label: "Aria - Tiếng Anh Mỹ (Nữ)"},
		{Value: "en-US-GuyNeural", Label: "Guy - Tiếng Anh Mỹ (Nam)"},
		{Value: "en-US-JennyNeural", Label: "Jenny - Tiếng Anh Mỹ (Nữ)"},
		{Value: "en-GB-SoniaNeural", Label: "Sonia - Tiếng Anh Anh (Nữ)"},
		{Value: "en-GB-RyanNeural", Label: "Ryan - Tiếng Anh Anh (Nam)"},
		// Tiếng Trung
		{Value: "zh-CN-XiaoxiaoNeural", Label: "Tiểu Tiểu (nữ)"},
		{Value: "zh-CN-YunxiNeural", Label: "Vân Hi (nam)"},
	},

	// Danh sách giọng Doubao TTS (Giao diện HTTP)
	// Tham khảo: https://www.volcengine.com/docs/6561/97465?lang=zh
	"doubao": {
		{Value: "BV700_V2_streaming", Label: "Xán Xán 2.0 (Nữ)"},
		{Value: "BV705_streaming", Label: "Dương Dương (Nữ)"},
		{Value: "BV701_V2_streaming", Label: "Kình Thương 2.0 (Nam)"},
		{Value: "BV001_V2_streaming", Label: "Giọng Nữ Phổ Thông 2.0"},
		{Value: "BV700_streaming", Label: "Xán Xán (Nữ)"},
		{Value: "BV406_V2_streaming", Label: "Siêu Tự Nhiên - Tử Tử 2.0 (Nữ)"},
		{Value: "BV406_streaming", Label: "Siêu Tự Nhiên - Tử Tử (Nữ)"},
		{Value: "BV407_V2_streaming", Label: "Siêu Tự Nhiên - Nhiên Nhiên 2.0 (Nữ)"},
		{Value: "BV407_streaming", Label: "Siêu Tự Nhiên - Nhiên Nhiên (Nữ)"},
		{Value: "BV001_streaming", Label: "Giọng Nữ Phổ Thông"},
		{Value: "BV002_streaming", Label: "Giọng Nam Phổ Thông"},
		{Value: "BV701_streaming", Label: "Kình Thương (Nam)"},
		{Value: "BV119_streaming", Label: "Chàng Rể Phổ Thông (Nam)"},
		{Value: "BV102_streaming", Label: "Thanh Niên Nho Nhã (Nam)"},
		{Value: "BV113_streaming", Label: "Thiếu Nữ Ngọt Ngào (Nữ)"},
		{Value: "BV115_streaming", Label: "Cổ Phong Thiếu Nữ (Nữ)"},
		{Value: "BV007_streaming", Label: "Giọng Nữ Thân Thiện"},
		{Value: "BV056_streaming", Label: "Giọng Nam Năng Động"},
		{Value: "BV005_streaming", Label: "Giọng Nữ Sôi Nổi"},
		{Value: "BV051_streaming", Label: "Trẻ Em Dễ Thương"},
		{Value: "BV034_streaming", Label: "Chị Tri Thức - Song Ngữ (Nữ)"},
		{Value: "BV033_streaming", Label: "Anh Ấm Áp (Nam)"},
		{Value: "BV021_streaming", Label: "Giọng Đông Bắc (Nam)"},
		{Value: "BV019_streaming", Label: "Giọng Trùng Khánh (Nam)"},
		{Value: "BV213_streaming", Label: "Giọng Quảng Tây (Nam)"},
		{Value: "BV503_streaming", Label: "Giọng Nữ Năng Động - Ariana"},
		{Value: "BV504_streaming", Label: "Giọng Nam Năng Động - Jackson"},
		{Value: "BV522_streaming", Label: "Giọng Nữ Thanh Lịch"},
		{Value: "BV524_streaming", Label: "Giọng Nam Tiếng Nhật"},
		{Value: "BV104_streaming", Label: "Thục Nữ Dịu Dàng (Nữ)"},
		{Value: "BV004_streaming", Label: "Thanh Niên Vui Vẻ (Nam)"},
		{Value: "BV009_streaming", Label: "Giọng Nữ Tri Thức"},
		{Value: "BV008_streaming", Label: "Giọng Nam Thân Thiện"},
		{Value: "BV064_streaming", Label: "Tiểu Thư Nhỏ (Nữ)"},
		{Value: "BV437_streaming", Label: "Bình Luận Viên - Đa Cảm Xúc (Nam)"},
		{Value: "BV511_streaming", Label: "Giọng Nữ Lười Biếng - Ava"},
		{Value: "BV040_streaming", Label: "Giọng Nữ Thân Thiện - Anna"},
		{Value: "BV138_streaming", Label: "Giọng Nữ Cảm Xúc - Lawrence"},
		{Value: "BV704_streaming", Label: "Xán Xán Phương Ngữ (Nữ)"},
		{Value: "BV702_streaming", Label: "Stefan (Nam)"},
		{Value: "BV421_streaming", Label: "Thiên Tài Thiếu Nữ (Nữ)"},
	},

	// Danh sách giọng Doubao WebSocket TTS
	// Tham khảo: https://www.volcengine.com/docs/6561/1257544?lang=zh
	// Danh sách này chỉ là các tùy chọn gợi ý, khả năng sử dụng thực tế
	// phụ thuộc vào appid/access_token đã được kích hoạt trên Volcengine Console.

	"doubao_ws": {
		// Giọng nữ
		{Value: "zh_female_cancan_mars_bigtts", Label: "Xán Xán / Shiny (Nữ)"},
		{Value: "zh_female_vv_uranus_bigtts", Label: "Vivi 2.0 (Nữ)"},
		{Value: "zh_female_vv_jupiter_bigtts", Label: "Vivi Bản O (Nữ)"},
		{Value: "zh_female_xiaohe_jupiter_bigtts", Label: "Tiểu Hà Bản O (Nữ)"},
		{Value: "saturn_zh_female_cancan_tob", Label: "Xán Xán Tri Thức (Nữ)"},
		{Value: "saturn_zh_female_keainvsheng_tob", Label: "Cô Gái Dễ Thương (Nữ)"},
		{Value: "saturn_zh_female_tiaopigongzhu_tob", Label: "Công Chúa Nghịch Ngợm (Nữ)"},
		{Value: "zh_female_xiaohe_uranus_bigtts", Label: "Tiểu Hà (Nữ)"},
		{Value: "zh_female_tianmeitaozi_mars_bigtts", Label: "Đào Ngọt Ngào (Nữ)"},
		{Value: "zh_female_wanwanxiaohe_moon_bigtts", Label: "Loan Loan Tiểu Hà (Nữ)"},
		{Value: "zh_female_qinqienvsheng_moon_bigtts", Label: "Giọng Nữ Thân Thiện (Nữ)"},
		{Value: "zh_female_vv_mars_bigtts", Label: "Vivi (Nữ)"},
		{Value: "zh_female_tianmeixiaoyuan_moon_bigtts", Label: "Tiểu Nguyên Ngọt Ngào (Nữ)"},
		{Value: "zh_female_qingchezizi_moon_bigtts", Label: "Tử Tử Trong Trẻo (Nữ)"},
		{Value: "zh_female_kailangjiejie_moon_bigtts", Label: "Chị Vui Vẻ (Nữ)"},
		{Value: "zh_female_tianmeiyueyue_moon_bigtts", Label: "Duyệt Duyệt Ngọt Ngào (Nữ)"},
		{Value: "zh_female_xinlingjitang_moon_bigtts", Label: "Giọng Nữ Truyền Cảm Hứng (Nữ)"},
		{Value: "zh_female_zhixingnvsheng_mars_bigtts", Label: "Giọng Nữ Tri Thức (Nữ)"},
		{Value: "zh_female_wenroushunv_mars_bigtts", Label: "Thục Nữ Dịu Dàng (Nữ)"},
		{Value: "zh_female_wenrouxiaoya_moon_bigtts", Label: "Tiểu Nhã Dịu Dàng (Nữ)"},
		{Value: "zh_female_linjianvhai_moon_bigtts", Label: "Cô Gái Láng Giềng (Nữ)"},
		{Value: "zh_female_shuangkuaisisi_moon_bigtts", Label: "Tư Tư Sảng Khoái / Skye (Nữ)"},
		{Value: "zh_female_gaolengyujie_moon_bigtts", Label: "Chị Lạnh Lùng Cao Quý (Nữ)"},
		{Value: "zh_female_meilinvyou_moon_bigtts", Label: "Bạn Gái Quyến Rũ (Nữ)"},
		{Value: "zh_female_sajiaonvyou_moon_bigtts", Label: "Bạn Gái Nũng Nịu (Nữ)"},
		{Value: "zh_female_yuanqinvyou_moon_bigtts", Label: "Em Gái Nũng Nịu (Nữ)"},
		{Value: "ICL_zh_female_wenrounvshen_239eff5e8ffa_tob", Label: "Nữ Thần Dịu Dàng (Nữ)"},
		{Value: "ICL_zh_female_chunzhenshaonv_e588402fb8ad_tob", Label: "Thiếu Nữ Thuần Khiết (Nữ)"},
		{Value: "ICL_zh_female_jinglingxiangdao_1beb294a9e3e_tob", Label: "Tinh Linh Hướng Dẫn (Nữ)"},
		{Value: "ICL_zh_female_yilin_tob", Label: "Em Gái Chu Đáo (Nữ)"},
		{Value: "ICL_zh_female_chengshujiejie_tob", Label: "Chị Trưởng Thành (Nữ)"},
		{Value: "ICL_zh_female_bingjiaojiejie_tob", Label: "Chị Bệnh Kiều (Nữ)"},
		{Value: "ICL_zh_female_wumeiyujie_tob", Label: "Chị Quyến Rũ (Nữ)"},
		{Value: "ICL_zh_female_aojiaonvyou_tob", Label: "Bạn Gái Tsundere (Nữ)"},
		{Value: "ICL_zh_female_tiexinnvyou_tob", Label: "Bạn Gái Chu Đáo (Nữ)"},
		{Value: "ICL_zh_female_xingganyujie_tob", Label: "Chị Gợi Cảm (Nữ)"},
		{Value: "ICL_zh_female_lixingyuanzi_cs_tob", Label: "Tổng Đài Viên Lý Trí (Nữ)"},
		{Value: "ICL_zh_female_wuxi_tob", Label: "Cô Nàng Ngọt Ngào Năng Động (Nữ)"},
		{Value: "ICL_zh_female_zhixingwenwan_tob", Label: "Tri Thức Ôn Nhu (Nữ)"},

		// Giọng nam
		{Value: "saturn_zh_male_shuanglangshaonian_tob", Label: "Thiếu Niên Sảng Khoái (Nam)"},
		{Value: "saturn_zh_male_tiancaitongzhuo_tob", Label: "Bạn Học Thiên Tài (Nam)"},
		{Value: "zh_male_yunzhou_jupiter_bigtts", Label: "Vân Chu Bản O (Nam)"},
		{Value: "zh_male_xiaotian_jupiter_bigtts", Label: "Tiểu Thiên Bản O (Nam)"},
		{Value: "zh_male_m191_uranus_bigtts", Label: "Vân Chu (Nam)"},
		{Value: "zh_male_taocheng_uranus_bigtts", Label: "Tiểu Thiên (Nam)"},
		{Value: "en_male_tim_uranus_bigtts", Label: "Tim (Giọng Nam Tiếng Anh)"},
		{Value: "zh_male_yangguangqingnian_moon_bigtts", Label: "Thanh Niên Năng Động (Nam)"},
		{Value: "zh_male_qingshuangnanda_mars_bigtts", Label: "Sinh Viên Nam Sảng Khoái (Nam)"},
		{Value: "zh_male_wenrouxiaoge_mars_bigtts", Label: "Anh Ấm Áp (Nam)"},
		{Value: "zh_male_qingcang_mars_bigtts", Label: "Kình Thương (Nam)"},
		{Value: "zh_male_ruyaqingnian_mars_bigtts", Label: "Thanh Niên Nho Nhã (Nam)"},
		{Value: "zh_male_jieshuoxiaoming_moon_bigtts", Label: "Bình Luận Viên Tiểu Minh (Nam)"},
		{Value: "zh_male_linjiananhai_moon_bigtts", Label: "Anh Láng Giềng (Nam)"},
		{Value: "zh_male_yuanboxiaoshu_moon_bigtts", Label: "Chú Uyên Bác (Nam)"},
		{Value: "zh_male_wennuanahu_moon_bigtts", Label: "A Hổ Ấm Áp / Alvin (Nam)"},
		{Value: "zh_male_shaonianzixin_moon_bigtts", Label: "Thiếu Niên Tử Tân / Brayan (Nam)"},
		{Value: "zh_male_beijingxiaoye_moon_bigtts", Label: "Thiếu Gia Bắc Kinh (Nam)"},
		{Value: "zh_male_jingqiangkanye_moon_bigtts", Label: "Phong Cách Bắc Kinh / Harmony (Nam)"},
		{Value: "zh_male_guozhoudege_moon_bigtts", Label: "Anh Đức Quảng Châu (Nam)"},
		{Value: "zh_male_haoyuxiaoge_moon_bigtts", Label: "Anh Hào Vũ (Nam)"},
		{Value: "zh_male_shenyeboke_moon_bigtts", Label: "Podcast Đêm Khuya (Nam)"},
		{Value: "zh_male_aojiaobazong_moon_bigtts", Label: "Tổng Tài Tsundere (Nam)"},
		{Value: "zh_male_dongfanghaoran_moon_bigtts", Label: "Đông Phương Hạo Nhiên (Nam)"},
		{Value: "zh_male_M100_conversation_wvae_bigtts", Label: "Quân Tử Ung Dung / Lucas (Nam)"},
		{Value: "zh_male_xudong_conversation_wvae_bigtts", Label: "Tiểu Đông Vui Vẻ / Daníel (Nam)"},
		{Value: "zh_male_qingyiyuxuan_mars_bigtts", Label: "A Thần Năng Động (Nam)"},
		{Value: "en_male_jason_conversation_wvae_bigtts", Label: "Anh Vui Vẻ (Nam)"},
		{Value: "ICL_zh_male_lengkugege_v1_tob", Label: "Anh Lạnh Lùng (Nam)"},
		{Value: "ICL_zh_male_shenmi_v1_tob", Label: "Chàng Trai Lanh Lợi (Nam)"},
		{Value: "ICL_zh_male_BV705_streaming_cs_tob", Label: "Dương Dương (Nam)"},
		{Value: "ICL_zh_male_menyoupingxiaoge_ffed9fc2fee7_tob", Label: "Anh Kiệm Lời (Nam)"},
		{Value: "ICL_zh_male_anrenqinzhu_cd62e63dcdab_tob", Label: "Tần Chủ Ám Nhẫn (Nam)"},
		{Value: "ICL_zh_male_guaogongzi_v1_tob", Label: "Công Tử Cô Ngạo (Nam)"},
		{Value: "ICL_zh_male_bingruogongzi_tob", Label: "Công Tử Yếu Đuối (Nam)"},
		{Value: "ICL_zh_male_bingjiaodidi_tob", Label: "Em Trai Bệnh Kiều (Nam)"},
		{Value: "ICL_zh_male_aomanshaoye_tob", Label: "Thiếu Gia Kiêu Ngạo (Nam)"},
		{Value: "ICL_zh_male_chunzhenxuedi_tob", Label: "Đàn Em Thuần Khiết (Nam)"},
		{Value: "ICL_zh_male_yourougongzi_tob", Label: "Công Tử Nhu Mì (Nam)"},
		{Value: "ICL_zh_male_tiexinnanyou_tob", Label: "Bạn Trai Chu Đáo (Nam)"},
		{Value: "ICL_zh_male_shaonianjiangjun_tob", Label: "少年将军（男声）"},
		{Value: "ICL_zh_male_bingjiaogege_tob", Label: "病娇哥哥（男声）"},
		{Value: "ICL_zh_male_xuebanantongzhuo_tob", Label: "学霸男同桌（男声）"},
		{Value: "ICL_zh_male_youmoshushu_tob", Label: "幽默叔叔（男声）"},
		{Value: "ICL_zh_male_wenrounantongzhuo_tob", Label: "温柔男同桌（男声）"},
		{Value: "ICL_zh_male_youmodaye_tob", Label: "幽默大爷（男声）"},
		{Value: "ICL_zh_male_shenmifashi_tob", Label: "神秘法师（男声）"},
		{Value: "ICL_zh_male_lengjunshangsi_tob", Label: "冷峻上司（男声）"},
		{Value: "ICL_en_male_michael_tob", Label: "Michael（美式英语男声）"},

		// IP/特色音色
		{Value: "zh_male_lubanqihao_mars_bigtts", Label: "鲁班七号（男声）"},
		{Value: "zh_female_yangmi_mars_bigtts", Label: "林潇（女声）"},
		{Value: "zh_female_linzhiling_mars_bigtts", Label: "玲玲姐姐（女声）"},
		{Value: "zh_female_jiyejizi2_mars_bigtts", Label: "春日部姐姐（女声）"},
		{Value: "zh_male_tangseng_mars_bigtts", Label: "唐僧（男声）"},
		{Value: "zh_male_zhubajie_mars_bigtts", Label: "猪八戒（男声）"},
		{Value: "zh_female_naying_mars_bigtts", Label: "直率英子（女声）"},
		{Value: "zh_female_leidian_mars_bigtts", Label: "女雷神（女声）"},
		{Value: "zh_male_sunwukong_mars_bigtts", Label: "猴哥（男声）"},
		{Value: "zh_male_xionger_mars_bigtts", Label: "熊二（男声）"},
		{Value: "zh_female_peiqi_mars_bigtts", Label: "佩奇猪（女声）"},
		{Value: "zh_female_yingtaowanzi_mars_bigtts", Label: "樱桃丸子（女声）"},
		{Value: "zh_male_silang_mars_bigtts", Label: "四郎（男声）"},
	},

	// Minimax TTS 音色列表
	// 参考：https://www.minimaxi.com/document/guides/tts-model
	"minimax": {
		// 中文 (普通话)
		{Value: "male-qn-qingse", Label: "青涩青年音色"},
		{Value: "male-qn-jingying", Label: "精英青年音色"},
		{Value: "male-qn-badao", Label: "霸道青年音色"},
		{Value: "male-qn-daxuesheng", Label: "青年大学生音色"},
		{Value: "female-shaonv", Label: "少女音色"},
		{Value: "female-yujie", Label: "御姐音色"},
		{Value: "female-chengshu", Label: "成熟女性音色"},
		{Value: "female-tianmei", Label: "甜美女性音色"},
		{Value: "male-qn-qingse-jingpin", Label: "青涩青年音色-beta"},
		{Value: "male-qn-jingying-jingpin", Label: "精英青年音色-beta"},
		{Value: "male-qn-badao-jingpin", Label: "霸道青年音色-beta"},
		{Value: "male-qn-daxuesheng-jingpin", Label: "青年大学生音色-beta"},
		{Value: "female-shaonv-jingpin", Label: "少女音色-beta"},
		{Value: "female-yujie-jingpin", Label: "御姐音色-beta"},
		{Value: "female-chengshu-jingpin", Label: "成熟女性音色-beta"},
		{Value: "female-tianmei-jingpin", Label: "甜美女性音色-beta"},
		{Value: "clever_boy", Label: "聪明男童"},
		{Value: "cute_boy", Label: "可爱男童"},
		{Value: "lovely_girl", Label: "萌萌女童"},
		{Value: "cartoon_pig", Label: "卡通猪小琪"},
		{Value: "bingjiao_didi", Label: "病娇弟弟"},
		{Value: "junlang_nanyou", Label: "俊朗男友"},
		{Value: "chunzhen_xuedi", Label: "纯真学弟"},
		{Value: "lengdan_xiongzhang", Label: "冷淡学长"},
		{Value: "badao_shaoye", Label: "霸道少爷"},
		{Value: "tianxin_xiaoling", Label: "甜心小玲"},
		{Value: "qiaopi_mengmei", Label: "俏皮萌妹"},
		{Value: "wumei_yujie", Label: "妩媚御姐"},
		{Value: "diadia_xuemei", Label: "嗲嗲学妹"},
		{Value: "danya_xuejie", Label: "淡雅学姐"},
		{Value: "Chinese (Mandarin)_Reliable_Executive", Label: "沉稳高管"},
		{Value: "Chinese (Mandarin)_News_Anchor", Label: "新闻女声"},
		{Value: "Chinese (Mandarin)_Mature_Woman", Label: "傲娇御姐"},
		{Value: "Chinese (Mandarin)_Unrestrained_Young_Man", Label: "不羁青年"},
		{Value: "Arrogant_Miss", Label: "嚣张小姐"},
		{Value: "Robot_Armor", Label: "机械战甲"},
		{Value: "Chinese (Mandarin)_Kind-hearted_Antie", Label: "热心大婶"},
		{Value: "Chinese (Mandarin)_HK_Flight_Attendant", Label: "港普空姐"},
		{Value: "Chinese (Mandarin)_Humorous_Elder", Label: "搞笑大爷"},
		{Value: "Chinese (Mandarin)_Gentleman", Label: "温润男声"},
		{Value: "Chinese (Mandarin)_Warm_Bestie", Label: "温暖闺蜜"},
		{Value: "Chinese (Mandarin)_Male_Announcer", Label: "播报男声"},
		{Value: "Chinese (Mandarin)_Sweet_Lady", Label: "甜美女声"},
		{Value: "Chinese (Mandarin)_Southern_Young_Man", Label: "南方小哥"},
		{Value: "Chinese (Mandarin)_Wise_Women", Label: "阅历姐姐"},
		{Value: "Chinese (Mandarin)_Gentle_Youth", Label: "温润青年"},
		{Value: "Chinese (Mandarin)_Warm_Girl", Label: "温暖少女"},
		{Value: "Chinese (Mandarin)_Kind-hearted_Elder", Label: "花甲奶奶"},
		{Value: "Chinese (Mandarin)_Cute_Spirit", Label: "憨憨萌兽"},
		{Value: "Chinese (Mandarin)_Radio_Host", Label: "电台男主播"},
		{Value: "Chinese (Mandarin)_Lyrical_Voice", Label: "抒情男声"},
		{Value: "Chinese (Mandarin)_Straightforward_Boy", Label: "率真弟弟"},
		{Value: "Chinese (Mandarin)_Sincere_Adult", Label: "真诚青年"},
		{Value: "Chinese (Mandarin)_Gentle_Senior", Label: "温柔学姐"},
		{Value: "Chinese (Mandarin)_Stubborn_Friend", Label: "嘴硬竹马"},
		{Value: "Chinese (Mandarin)_Crisp_Girl", Label: "清脆少女"},
		{Value: "Chinese (Mandarin)_Pure-hearted_Boy", Label: "清澈邻家弟弟"},
		{Value: "Chinese (Mandarin)_Soft_Girl", Label: "柔和少女"},
		// 中文 (粤语)
		{Value: "Cantonese_ProfessionalHost（F)", Label: "专业女主持"},
		{Value: "Cantonese_GentleLady", Label: "温柔女声"},
		{Value: "Cantonese_ProfessionalHost（M)", Label: "专业男主持"},
		{Value: "Cantonese_PlayfulMan", Label: "活泼男声"},
		{Value: "Cantonese_CuteGirl", Label: "可爱女孩"},
		{Value: "Cantonese_KindWoman", Label: "善良女声"},
		// 英文
		{Value: "Santa_Claus", Label: "Santa Claus"},
		{Value: "Grinch", Label: "Grinch"},
		{Value: "Rudolph", Label: "Rudolph"},
		{Value: "Arnold", Label: "Arnold"},
		{Value: "Charming_Santa", Label: "Charming Santa"},
		{Value: "Charming_Lady", Label: "Charming Lady"},
		{Value: "Sweet_Girl", Label: "Sweet Girl"},
		{Value: "Cute_Elf", Label: "Cute Elf"},
		{Value: "Attractive_Girl", Label: "Attractive Girl"},
		{Value: "Serene_Woman", Label: "Serene Woman"},
		{Value: "English_Trustworthy_Man", Label: "Trustworthy Man"},
		{Value: "English_Graceful_Lady", Label: "Graceful Lady"},
		{Value: "English_Aussie_Bloke", Label: "Aussie Bloke"},
		{Value: "English_Whispering_girl", Label: "Whispering girl"},
		{Value: "English_Diligent_Man", Label: "Diligent Man"},
		{Value: "English_Gentle-voiced_man", Label: "Gentle-voiced man"},
	},

	// Danh sách giọng Aliyun Qwen TTS (danh sách cơ bản, lọc theo model bởi GetAliyunQwenVoicesByModel)
	"aliyun_qwen": {
		{Value: "Cherry", Label: "Thiên Duyệt (Nữ)"},
		{Value: "Serena", Label: "Tô Dao (Nữ)"},
		{Value: "Ethan", Label: "Thần Húc (Nam)"},
		{Value: "Chelsie", Label: "Thiên Tuyết (Nữ)"},
		{Value: "Momo", Label: "Mạt Thỏ (Nữ)"},
		{Value: "Vivian", Label: "Thập Tam (Nữ)"},
		{Value: "Moon", Label: "Nguyệt Bạch (Nam)"},
		{Value: "Maia", Label: "Tứ Nguyệt (Nữ)"},
		{Value: "Kai", Label: "Khải (Nam)"},
		{Value: "Nofish", Label: "Không Ăn Cá (Nam)"},
		{Value: "Bella", Label: "Em Bé Dễ Thương (Nữ)"},
		{Value: "Jennifer", Label: "Jennifer (Nữ)"},
		{Value: "Ryan", Label: "Trà Ngọt (Nam)"},
	},

	// Danh sách giọng iFlytek TTS trực tuyến
	// Lưu ý: đây là danh sách giọng tĩnh phổ biến, khả năng sử dụng thực tế phụ thuộc vào quyền được cấp trên console iFlytek.
	// Tham khảo:
	// https://www.xfyun.cn/doc/tts/online_tts/API.html
	// https://aiui.xfyun.cn/doc/aiui/3_access_service/access_interact/functions/speech_synthesis.html
	"xunfei": {
		{Value: "xiaoyan", Label: "Tiểu Yến (Nữ, Mặc Định)"},
		{Value: "xiaofeng", Label: "Hiểu Phong (Nam)"},
		{Value: "yezi", Label: "Tiểu Lộ (Nữ)"},
		{Value: "yifei", Label: "Nhất Phi (Nữ)"},
		{Value: "yiping", Label: "Nhất Bình (Nữ)"},
		{Value: "qige", Label: "Anh Bảy (Nam)"},
		{Value: "chaoge", Label: "Anh Siêu (Nam)"},
		{Value: "pengfei", Label: "Tiểu Bằng (Nam)"},
		{Value: "xiaoxin", Label: "Tiểu Tân Dễ Thương (Giọng Trẻ Em)"},
		{Value: "john", Label: "John (Giọng Nam Tiếng Anh)"},
		{Value: "catherine", Label: "Catherine (Giọng Nữ Tiếng Anh)"},
	},

	// Danh sách giọng iFlytek Super TTS (siêu thực)
	// Lưu ý: danh sách tĩnh được đề xuất, khả năng sử dụng phụ thuộc vào quyền trên console iFlytek.
	"xunfei_super_tts": {
		{Value: "x6_lingxiaoxue_pro", Label: "Linh Tiểu Tuyết (x6)"},
		{Value: "x6_lingfeiyi_pro", Label: "Linh Phi Dật (x6)"},
		{Value: "x6_lingxiaoli_pro", Label: "Linh Tiểu Lệ (x6)"},
		{Value: "x6_lingxiaoyue_pro", Label: "Linh Tiểu Nguyệt (x6)"},
		{Value: "x6_lingxiaoxuan_pro", Label: "Linh Tiểu Huyên (x6)"},
		{Value: "x6_lingyuyan_pro", Label: "Linh Ngữ Yên (x6)"},
		{Value: "x6_lingyouyou_pro", Label: "Linh Du Du (x6)"},
		{Value: "x6_feizheChat_pro", Label: "Phi Triết Chat (x6)"},
		{Value: "x6_xiaoqiChat_pro", Label: "Tiểu Kỳ Chat (x6)"},
		{Value: "x5_lingxiaotang_flow", Label: "Linh Tiểu Đường (x5)"},
		{Value: "x5_lingyuzhao_flow", Label: "Linh Ngữ Chiêu (x5)"},
		{Value: "x4_zijin_oral", Label: "Tử Cẩn (x4, Khẩu Ngữ)"},
		{Value: "x4_ziyang_oral", Label: "Tử Dương (x4, Khẩu Ngữ)"},
	},

	// Danh sách giọng Zhipu TTS
	"zhipu": {
		{Value: "tongtong", Label: "Đồng Đồng (Mặc Định)"},
		{Value: "chuichui", Label: "Chùy Chùy"},
		{Value: "xiaochen", Label: "Tiểu Trần"},
		{Value: "jam", Label: "Động Vật Jam"},
		{Value: "kazi", Label: "Động Vật Kazi"},
		{Value: "douji", Label: "Động Vật Douji"},
		{Value: "luodo", Label: "Động Vật Luodo"},
	},
}

// GetVoiceOptionsByProvider lấy danh sách giọng theo provider
func GetVoiceOptionsByProvider(provider string) []VoiceOption {
	if voices, ok := VoiceOptions[provider]; ok {
		return voices
	}
	return []VoiceOption{}
}

// GetAliyunQwenVoicesByModel lấy danh sách giọng theo tên model Qwen
// Sử dụng bản đồ model trong package qwen để lấy danh sách giọng chính xác
func GetAliyunQwenVoicesByModel(model string) []VoiceOption {
	model = strings.TrimSpace(model)
	if model == "" {
		// Nếu không có model, trả về danh sách cơ bản
		return GetVoiceOptionsByProvider("aliyun_qwen")
	}

	// Sử dụng hàm cục bộ để lấy danh sách giọng theo model
	voices := GetVoicesByModel(model)
	if voices == nil || len(voices) == 0 {
		// Nếu không tìm thấy giọng cho model, trả về danh sách cơ bản
		return GetVoiceOptionsByProvider("aliyun_qwen")
	}

	// Chuyển đổi VoiceInfo sang VoiceOption
	result := make([]VoiceOption, 0, len(voices))
	for _, v := range voices {
		result = append(result, VoiceOption{
			Value: v.Value,
			Label: v.Label,
		})
	}
	return result
}