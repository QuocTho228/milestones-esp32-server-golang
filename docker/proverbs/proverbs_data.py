# -*- coding: utf-8 -*-
"""
100 câu ca dao, tục ngữ phổ biến của Việt Nam, kèm giải nghĩa ngắn gọn và
nhãn chủ đề (themes) để hỗ trợ tìm kiếm theo chủ đề (ví dụ: "biết ơn",
"hiếu thảo", "chăm chỉ"...).

Đây là kho tàng văn học dân gian truyền miệng, không có tác giả cụ thể
(khuyết danh), thuộc phạm vi công cộng.
"""

PROVERBS = [
    # ---------------- Biết ơn ----------------
    {"text": "Ăn quả nhớ kẻ trồng cây", "meaning": "Khi được hưởng thành quả, phải nhớ ơn người đã tạo ra thành quả đó.", "themes": ["biết ơn"]},
    {"text": "Uống nước nhớ nguồn", "meaning": "Được hưởng lợi ích gì thì phải nhớ đến nguồn gốc, người đã tạo ra lợi ích ấy.", "themes": ["biết ơn"]},
    {"text": "Ăn cây nào rào cây ấy", "meaning": "Nhận ơn nghĩa hay lợi ích từ đâu thì phải có trách nhiệm bảo vệ, gắn bó với nơi đó.", "themes": ["biết ơn", "đạo đức"]},
    {"text": "Ăn gạo nhớ kẻ đâm xay giần sàng", "meaning": "Được ăn hạt gạo trắng phải nhớ công lao người làm ra nó, nhắc nhở biết ơn người lao động.", "themes": ["biết ơn"]},
    {"text": "Ăn khoai nhớ kẻ cho dây mà trồng", "meaning": "Hưởng thành quả phải nhớ người đã giúp đỡ, tạo điều kiện ban đầu.", "themes": ["biết ơn"]},
    {"text": "Uống nước nhớ người đào giếng", "meaning": "Được hưởng thụ thành quả phải khắc ghi công lao của người đã tạo ra nó.", "themes": ["biết ơn"]},
    {"text": "Công cha như núi Thái Sơn, nghĩa mẹ như nước trong nguồn chảy ra", "meaning": "Công lao sinh thành dưỡng dục của cha mẹ vô cùng to lớn, sâu nặng như núi cao, nguồn nước không bao giờ cạn.", "themes": ["biết ơn", "hiếu thảo", "gia đình"]},

    # ---------------- Hiếu thảo ----------------
    {"text": "Công cha nghĩa mẹ ơn thầy", "meaning": "Nhắc con người phải ghi nhớ và đền đáp ba công ơn lớn: cha sinh thành, mẹ nuôi dưỡng, thầy dạy dỗ.", "themes": ["hiếu thảo", "biết ơn"]},
    {"text": "Cá không ăn muối cá ươn, con cưỡng cha mẹ trăm đường con hư", "meaning": "Con cái không nghe lời dạy bảo của cha mẹ thì dễ hư hỏng, giống như cá không ướp muối sẽ bị ươn.", "themes": ["hiếu thảo"]},
    {"text": "Đi khắp thế gian không ai tốt bằng mẹ, gánh nặng cuộc đời không ai khổ bằng cha", "meaning": "Ca ngợi tình yêu thương vô bờ của mẹ và sự vất vả, hy sinh thầm lặng của cha.", "themes": ["hiếu thảo", "gia đình"]},
    {"text": "Một lòng thờ mẹ kính cha, cho tròn chữ hiếu mới là đạo con", "meaning": "Phận làm con phải trọn đạo hiếu, hết lòng phụng dưỡng, kính trọng cha mẹ.", "themes": ["hiếu thảo"]},
    {"text": "Cha mẹ nuôi con biển hồ lai láng, con nuôi cha mẹ tính tháng tính ngày", "meaning": "Phê phán sự so sánh: cha mẹ nuôi con vô điều kiện, không tính toán, còn con cái nuôi cha mẹ già lại hay tính toán thiệt hơn.", "themes": ["hiếu thảo"]},

    # ---------------- Chăm chỉ, cần cù ----------------
    {"text": "Có công mài sắt, có ngày nên kim", "meaning": "Kiên trì, chăm chỉ làm việc dù khó khăn đến đâu cũng sẽ đạt được thành công.", "themes": ["chăm chỉ", "kiên trì"]},
    {"text": "Kiến tha lâu cũng đầy tổ", "meaning": "Chăm chỉ, kiên trì tích lũy từng chút một lâu ngày cũng sẽ có thành quả lớn.", "themes": ["chăm chỉ", "kiên trì", "tiết kiệm"]},
    {"text": "Tay làm hàm nhai, tay quai miệng trễ", "meaning": "Có chịu khó lao động thì mới có cái ăn, lười biếng thì phải chịu đói.", "themes": ["chăm chỉ"]},
    {"text": "Cần cù bù thông minh", "meaning": "Nếu không có sẵn sự thông minh, chỉ cần chăm chỉ, chịu khó cũng có thể đạt kết quả tốt.", "themes": ["chăm chỉ", "học tập"]},
    {"text": "Năng nhặt chặt bị", "meaning": "Chăm chỉ nhặt nhạnh, tích góp từng chút một thì cuối cùng sẽ được nhiều.", "themes": ["chăm chỉ", "tiết kiệm"]},
    {"text": "Siêng làm thì có, siêng học thì hay", "meaning": "Chăm chỉ lao động thì có của cải, chăm chỉ học tập thì giỏi giang.", "themes": ["chăm chỉ", "học tập"]},
    {"text": "Đổ mồ hôi, sôi nước mắt", "meaning": "Diễn tả sự vất vả, cực nhọc trong lao động để đạt được thành quả.", "themes": ["chăm chỉ"]},

    # ---------------- Kiên trì, nỗ lực ----------------
    {"text": "Có chí thì nên", "meaning": "Có ý chí, quyết tâm thì việc gì cũng có thể làm thành công.", "themes": ["kiên trì"]},
    {"text": "Thất bại là mẹ thành công", "meaning": "Những lần thất bại là bài học quý giá giúp con người rút kinh nghiệm để đi đến thành công.", "themes": ["kiên trì"]},
    {"text": "Có công mài sắt có ngày nên kim", "meaning": "Kiên nhẫn, bền bỉ làm việc khó khăn đến mấy cũng sẽ đạt kết quả.", "themes": ["kiên trì", "chăm chỉ"]},
    {"text": "Nước chảy đá mòn", "meaning": "Kiên trì, bền bỉ theo thời gian có thể làm thay đổi cả những điều tưởng chừng không thể.", "themes": ["kiên trì"]},
    {"text": "Còn nước còn tát", "meaning": "Còn cơ hội, còn khả năng thì vẫn phải cố gắng hết sức, không bỏ cuộc.", "themes": ["kiên trì"]},
    {"text": "Thua keo này, bày keo khác", "meaning": "Thất bại lần này thì thử lại lần khác, không nản chí bỏ cuộc.", "themes": ["kiên trì"]},

    # ---------------- Đoàn kết ----------------
    {"text": "Một cây làm chẳng nên non, ba cây chụm lại nên hòn núi cao", "meaning": "Sức mạnh của tập thể, đoàn kết luôn lớn hơn sức mạnh của một cá nhân đơn lẻ.", "themes": ["đoàn kết"]},
    {"text": "Đoàn kết là sức mạnh", "meaning": "Khi mọi người cùng chung sức, đồng lòng thì sẽ tạo nên sức mạnh to lớn để vượt qua khó khăn.", "themes": ["đoàn kết"]},
    {"text": "Lá lành đùm lá rách", "meaning": "Người có điều kiện tốt hơn nên giúp đỡ, che chở người khó khăn hơn mình.", "themes": ["đoàn kết", "tình người"]},
    {"text": "Bầu ơi thương lấy bí cùng, tuy rằng khác giống nhưng chung một giàn", "meaning": "Mọi người tuy khác nhau về hoàn cảnh, xuất thân nhưng cùng sống chung một cộng đồng thì nên yêu thương, đùm bọc nhau.", "themes": ["đoàn kết", "tình người"]},
    {"text": "Chị ngã em nâng", "meaning": "Anh chị em trong nhà phải biết yêu thương, giúp đỡ, nâng đỡ nhau lúc khó khăn.", "themes": ["đoàn kết", "gia đình"]},
    {"text": "Ngựa chạy có bầy, chim bay có bạn", "meaning": "Con người sống cần có tập thể, bạn bè để nương tựa, hỗ trợ lẫn nhau.", "themes": ["đoàn kết", "tình bạn"]},

    # ---------------- Trung thực ----------------
    {"text": "Ăn ngay nói thật, mọi tật mọi lành", "meaning": "Sống ngay thẳng, thật thà thì dù có gặp khó khăn oan uổng gì cũng sẽ được yên ổn.", "themes": ["trung thực", "đạo đức"]},
    {"text": "Thật thà là cha quỷ quái", "meaning": "Sự thật thà, chân thành cuối cùng sẽ thắng được những mưu mô, xảo trá.", "themes": ["trung thực"]},
    {"text": "Cây ngay không sợ chết đứng", "meaning": "Người sống ngay thẳng, trong sạch thì không sợ bị vu oan hay chỉ trích.", "themes": ["trung thực", "đạo đức"]},
    {"text": "Giấy rách phải giữ lấy lề", "meaning": "Dù nghèo khó, sa sút đến đâu cũng phải giữ gìn phẩm giá, nề nếp, đạo đức làm người.", "themes": ["trung thực", "đạo đức"]},
    {"text": "Một lần bất tín, vạn lần bất tin", "meaning": "Chỉ cần một lần thất hứa, nói dối thì sẽ mất lòng tin của người khác mãi mãi.", "themes": ["trung thực"]},

    # ---------------- Tiết kiệm ----------------
    {"text": "Tích tiểu thành đại", "meaning": "Tích lũy những điều nhỏ bé lâu dần sẽ trở thành thành quả lớn.", "themes": ["tiết kiệm", "chăm chỉ"]},
    {"text": "Khéo ăn thì no, khéo co thì ấm", "meaning": "Biết chi tiêu, sắp xếp hợp lý thì dù ít của cải cũng đủ sống thoải mái.", "themes": ["tiết kiệm"]},
    {"text": "Ăn phải dành, có phải kiệm", "meaning": "Phải biết dành dụm khi ăn uống, biết tiết kiệm khi có của cải để phòng khi khó khăn.", "themes": ["tiết kiệm"]},
    {"text": "Buôn tàu bán bè không bằng ăn dè hà tiện", "meaning": "Dù buôn bán lớn đến đâu mà tiêu xài hoang phí thì cũng khó giàu; biết tiết kiệm mới là gốc của sự giàu có.", "themes": ["tiết kiệm"]},
    {"text": "Năng nhặt chặt bị", "meaning": "Chăm chỉ tiết kiệm, tích góp từng chút nhỏ rồi cũng sẽ đầy.", "themes": ["tiết kiệm", "chăm chỉ"]},

    # ---------------- Học tập ----------------
    {"text": "Học ăn, học nói, học gói, học mở", "meaning": "Con người cần phải học hỏi mọi điều trong cuộc sống, từ cách ăn nói đến cách cư xử, làm việc.", "themes": ["học tập"]},
    {"text": "Không thầy đố mày làm nên", "meaning": "Đề cao vai trò của người thầy trong việc dạy dỗ, không có thầy chỉ dạy thì khó thành công.", "themes": ["học tập"]},
    {"text": "Học thầy không tày học bạn", "meaning": "Ngoài học từ thầy, việc học hỏi lẫn nhau giữa bạn bè cũng rất quan trọng và hiệu quả.", "themes": ["học tập", "tình bạn"]},
    {"text": "Đi một ngày đàng, học một sàng khôn", "meaning": "Đi ra ngoài, trải nghiệm nhiều sẽ học hỏi, tích lũy được nhiều kiến thức và kinh nghiệm sống.", "themes": ["học tập"]},
    {"text": "Dốt đến đâu học lâu cũng biết", "meaning": "Dù chưa giỏi, chưa biết nhưng nếu kiên trì học tập lâu dài thì cuối cùng cũng sẽ hiểu biết, thành thạo.", "themes": ["học tập", "kiên trì"]},

    # ---------------- Khiêm tốn ----------------
    {"text": "Khiêm tốn bao nhiêu cũng chưa đủ, tự kiêu một chút cũng là thừa", "meaning": "Con người nên luôn khiêm tốn học hỏi, tránh tự cao tự đại dù chỉ một chút.", "themes": ["khiêm tốn"]},
    {"text": "Ếch ngồi đáy giếng, coi trời bằng vung", "meaning": "Phê phán người hiểu biết hạn hẹp nhưng lại tự cho mình là hiểu biết rộng, kiêu ngạo.", "themes": ["khiêm tốn"]},
    {"text": "Núi cao còn có núi cao hơn", "meaning": "Dù giỏi đến đâu cũng luôn có người giỏi hơn, nhắc nhở con người không nên tự mãn.", "themes": ["khiêm tốn"]},
    {"text": "Biết thì thưa thốt, không biết thì dựa cột mà nghe", "meaning": "Chỉ nên nói khi thực sự hiểu biết về điều đó, không nên tỏ ra hiểu biết khi mình không biết.", "themes": ["khiêm tốn"]},

    # ---------------- Tình bạn, tình người ----------------
    {"text": "Thương người như thể thương thân", "meaning": "Nên yêu thương, đối xử tốt với người khác giống như yêu thương chính bản thân mình.", "themes": ["tình người", "đạo đức"]},
    {"text": "Một con ngựa đau, cả tàu bỏ cỏ", "meaning": "Khi một người trong tập thể gặp khó khăn, hoạn nạn thì cả tập thể cùng lo lắng, chia sẻ.", "themes": ["tình người", "đoàn kết"]},
    {"text": "Ăn ở có nhân, mười phần chẳng khó", "meaning": "Sống nhân hậu, có lòng tốt với người khác thì cuộc sống sẽ dễ dàng, thuận lợi hơn.", "themes": ["tình người", "đạo đức"]},
    {"text": "Cứu một mạng người hơn xây bảy tháp phù đồ", "meaning": "Việc cứu giúp mạng sống con người là công đức lớn hơn cả việc xây dựng chùa tháp.", "themes": ["tình người", "đạo đức"]},
    {"text": "Giàu vì bạn, sang vì vợ", "meaning": "Có được bạn tốt và người vợ (chồng) tốt là một loại giàu sang quý giá trong đời.", "themes": ["tình bạn", "gia đình"]},

    # ---------------- Cẩn thận, thận trọng ----------------
    {"text": "Cẩn tắc vô áy náy", "meaning": "Cẩn thận, chu đáo trong mọi việc thì sẽ không phải lo lắng, hối tiếc về sau.", "themes": ["thận trọng"]},
    {"text": "Đi đêm lắm có ngày gặp ma", "meaning": "Làm việc mờ ám, liều lĩnh nhiều lần thì sớm muộn cũng gặp rắc rối, hậu quả xấu.", "themes": ["thận trọng", "nhân quả"]},
    {"text": "Ăn cỗ đi trước, lội nước đi sau", "meaning": "Phê phán những người khôn lỏi, chỉ giành phần lợi cho mình mà tránh né việc khó, nguy hiểm.", "themes": ["thận trọng"]},
    {"text": "Sai một li, đi một dặm", "meaning": "Chỉ cần sai lệch một chút ban đầu, kết quả cuối cùng có thể lệch đi rất xa; nhắc nhở cẩn thận, chính xác.", "themes": ["thận trọng"]},
    {"text": "Đo bò làm chuồng", "meaning": "Trước khi làm việc gì cần tính toán, cân nhắc kỹ lưỡng cho phù hợp với thực tế.", "themes": ["thận trọng"]},

    # ---------------- Nhân quả ----------------
    {"text": "Gieo gió gặt bão", "meaning": "Làm điều xấu, gây hại cho người khác thì sẽ phải nhận hậu quả nặng nề tương xứng.", "themes": ["nhân quả"]},
    {"text": "Ở hiền gặp lành", "meaning": "Sống hiền lành, tốt bụng thì sẽ gặp được những điều may mắn, tốt đẹp trong cuộc sống.", "themes": ["nhân quả", "đạo đức"]},
    {"text": "Ác giả ác báo", "meaning": "Người làm điều ác thì sớm muộn cũng sẽ phải gánh chịu hậu quả xấu do chính mình gây ra.", "themes": ["nhân quả"]},
    {"text": "Gieo nhân nào, gặt quả nấy", "meaning": "Hành động của con người hôm nay sẽ quyết định kết quả mà họ nhận được sau này.", "themes": ["nhân quả"]},

    # ---------------- Gia đình ----------------
    {"text": "Con hơn cha là nhà có phúc", "meaning": "Khi con cái giỏi giang, thành đạt hơn cha mẹ thì đó là niềm phúc lớn cho cả gia đình.", "themes": ["gia đình"]},
    {"text": "Thuận vợ thuận chồng, tát biển Đông cũng cạn", "meaning": "Vợ chồng nếu đồng lòng, hòa thuận thì có thể vượt qua mọi khó khăn dù lớn đến đâu.", "themes": ["gia đình", "đoàn kết"]},
    {"text": "Anh em như thể tay chân", "meaning": "Anh chị em ruột thịt gắn bó khăng khít, không thể tách rời như tay với chân trên cùng một cơ thể.", "themes": ["gia đình"]},
    {"text": "Con dại cái mang", "meaning": "Con cái làm điều sai trái thì cha mẹ cũng phải chịu trách nhiệm, gánh vác cùng.", "themes": ["gia đình"]},
    {"text": "Có nuôi con mới biết lòng cha mẹ", "meaning": "Chỉ khi tự mình làm cha mẹ, nuôi dạy con cái mới thực sự thấu hiểu được sự vất vả, tình yêu của cha mẹ mình.", "themes": ["gia đình", "hiếu thảo"]},

    # ---------------- Thời gian quý giá ----------------
    {"text": "Thời gian là vàng bạc", "meaning": "Thời gian quý giá như vàng bạc, cần biết trân trọng và sử dụng hợp lý, không lãng phí.", "themes": ["thời gian"]},
    {"text": "Nước đến chân mới nhảy", "meaning": "Phê phán thói quen chần chừ, chỉ hành động khi việc đã cấp bách, không chuẩn bị trước.", "themes": ["thời gian", "thận trọng"]},
    {"text": "Mất bò mới lo làm chuồng", "meaning": "Chỉ khi đã chịu thiệt hại rồi mới lo phòng ngừa, phê phán sự chủ quan, thiếu chuẩn bị trước.", "themes": ["thời gian", "thận trọng"]},
    {"text": "Nhất thì nhì thục", "meaning": "Trong việc nhà nông, chọn đúng thời điểm (thời vụ) quan trọng hơn cả việc đất đai màu mỡ; nhấn mạnh giá trị của đúng thời điểm.", "themes": ["thời gian", "kinh nghiệm dân gian"]},

    # ---------------- Tình yêu quê hương ----------------
    {"text": "Ta về ta tắm ao ta, dù trong dù đục ao nhà vẫn hơn", "meaning": "Dù quê hương, đất nước mình còn nghèo khó, vẫn luôn yêu quý và gắn bó hơn nơi khác.", "themes": ["quê hương"]},
    {"text": "Quê hương là chùm khế ngọt", "meaning": "Quê hương gắn liền với những kỷ niệm tuổi thơ ngọt ngào, thân thương, khó quên.", "themes": ["quê hương"]},
    {"text": "Anh đi anh nhớ quê nhà, nhớ canh rau muống nhớ cà dầm tương", "meaning": "Nỗi nhớ quê hương gắn liền với những món ăn dân dã, bình dị nhưng đầy thân thương.", "themes": ["quê hương"]},
    {"text": "Chim có tổ, người có tông", "meaning": "Con người ai cũng có nguồn gốc, quê hương, dòng tộc của mình, cần nhớ về cội nguồn.", "themes": ["quê hương", "biết ơn"]},

    # ---------------- Kinh nghiệm dân gian (thời tiết, lao động) ----------------
    {"text": "Chuồn chuồn bay thấp thì mưa, bay cao thì nắng, bay vừa thì râm", "meaning": "Kinh nghiệm dân gian dự đoán thời tiết dựa vào độ cao bay của chuồn chuồn.", "themes": ["kinh nghiệm dân gian"]},
    {"text": "Trăng quầng thì hạn, trăng tán thì mưa", "meaning": "Kinh nghiệm dự báo thời tiết dựa vào quầng sáng quanh mặt trăng.", "themes": ["kinh nghiệm dân gian"]},
    {"text": "Chớp đông nhay nháy, gà gáy thì mưa", "meaning": "Kinh nghiệm dân gian dự đoán trời sắp mưa dựa vào hiện tượng chớp ở hướng đông vào lúc gà gáy.", "themes": ["kinh nghiệm dân gian"]},
    {"text": "Tấc đất tấc vàng", "meaning": "Đất đai quý giá như vàng, nhắc nhở con người trân trọng, sử dụng đất đai hiệu quả.", "themes": ["kinh nghiệm dân gian", "chăm chỉ"]},
    {"text": "Nhất nước, nhì phân, tam cần, tứ giống", "meaning": "Kinh nghiệm canh tác nông nghiệp: nước tưới quan trọng nhất, sau đó đến phân bón, sự chăm chỉ và cuối cùng là giống cây trồng.", "themes": ["kinh nghiệm dân gian", "chăm chỉ"]},

    # ---------------- Ứng xử khôn ngoan / đạo đức làm người ----------------
    {"text": "Lời nói chẳng mất tiền mua, lựa lời mà nói cho vừa lòng nhau", "meaning": "Nói năng khéo léo, dễ nghe không tốn kém gì nhưng lại giúp mối quan hệ tốt đẹp hơn.", "themes": ["ứng xử", "đạo đức"]},
    {"text": "Uốn lưỡi bảy lần trước khi nói", "meaning": "Nên suy nghĩ kỹ càng, thận trọng trước khi phát ngôn để tránh gây tổn thương hay hiểu lầm.", "themes": ["ứng xử", "thận trọng"]},
    {"text": "Im lặng là vàng", "meaning": "Đôi khi giữ im lặng, không nói nhiều lại là cách ứng xử khôn ngoan, tránh rắc rối.", "themes": ["ứng xử"]},
    {"text": "Một điều nhịn, chín điều lành", "meaning": "Biết nhường nhịn, kiềm chế trong tranh cãi sẽ giúp tránh được nhiều điều không hay, giữ được hòa khí.", "themes": ["ứng xử", "đạo đức"]},
    {"text": "Lạt mềm buộc chặt", "meaning": "Cách ứng xử mềm mỏng, nhẹ nhàng đôi khi lại hiệu quả và bền vững hơn cách cứng rắn.", "themes": ["ứng xử"]},
    {"text": "Ở bầu thì tròn, ở ống thì dài", "meaning": "Con người dễ bị ảnh hưởng, thay đổi theo môi trường, hoàn cảnh sống xung quanh.", "themes": ["ứng xử", "đạo đức"]},
    {"text": "Gần mực thì đen, gần đèn thì sáng", "meaning": "Môi trường và những người xung quanh có ảnh hưởng lớn đến tính cách, phẩm chất của một người.", "themes": ["ứng xử", "đạo đức"]},
    {"text": "Đói cho sạch, rách cho thơm", "meaning": "Dù nghèo khó, thiếu thốn đến đâu cũng phải giữ gìn phẩm giá, nhân cách trong sạch.", "themes": ["đạo đức"]},
    {"text": "Cái nết đánh chết cái đẹp", "meaning": "Đức hạnh, tính cách tốt đẹp của con người quan trọng và có giá trị hơn vẻ đẹp bề ngoài.", "themes": ["đạo đức"]},
    {"text": "Tốt gỗ hơn tốt nước sơn", "meaning": "Chất lượng, bản chất bên trong quan trọng hơn hình thức, vẻ ngoài hào nhoáng.", "themes": ["đạo đức"]},
    {"text": "Giấy rách phải giữ lấy lề", "meaning": "Dù hoàn cảnh khó khăn, sa sút vẫn phải giữ được nề nếp, gia phong, phẩm chất tốt đẹp.", "themes": ["đạo đức"]},
    {"text": "Chớ thấy sóng cả mà ngã tay chèo", "meaning": "Dù gặp khó khăn, thử thách lớn cũng không được nản lòng, bỏ cuộc giữa chừng.", "themes": ["kiên trì"]},
    {"text": "Muốn ăn phải lăn vào bếp", "meaning": "Muốn có được thành quả thì phải tự mình bắt tay vào làm việc, không thể trông chờ vào người khác.", "themes": ["chăm chỉ"]},
    {"text": "Có làm thì mới có ăn, không dưng ai dễ đem phần đến cho", "meaning": "Phải tự lao động thì mới có thành quả để hưởng, không ai tự nhiên cho không mình điều gì.", "themes": ["chăm chỉ"]},
    {"text": "Miệng nam mô, bụng bồ dao găm", "meaning": "Phê phán những người bề ngoài tỏ ra hiền lành, đạo đức nhưng trong lòng lại nham hiểm, xấu xa.", "themes": ["đạo đức", "ứng xử"]},
    {"text": "Ăn cháo đá bát", "meaning": "Phê phán những người vô ơn, đã được giúp đỡ nhưng lại quay lưng phản bội người đã giúp mình.", "themes": ["đạo đức", "biết ơn"]},
    {"text": "Chọn bạn mà chơi, chọn nơi mà ở", "meaning": "Cần cẩn thận lựa chọn bạn bè và môi trường sống vì chúng có ảnh hưởng lớn đến bản thân.", "themes": ["ứng xử", "tình bạn"]},
    {"text": "Gần chùa gọi bụt bằng anh", "meaning": "Phê phán việc vì quá thân quen mà sinh ra thiếu tôn trọng, coi thường người trên, người có uy tín.", "themes": ["ứng xử"]},
    {"text": "Tránh voi chẳng xấu mặt nào", "meaning": "Việc nhường nhịn, tránh đối đầu với người/việc mạnh hơn mình không phải là điều đáng xấu hổ.", "themes": ["ứng xử", "thận trọng"]},
    {"text": "Mất lòng trước, được lòng sau", "meaning": "Đôi khi nói thẳng, làm rõ ràng ngay từ đầu dù có thể mất lòng nhưng lại giúp mối quan hệ bền vững, tốt đẹp về sau.", "themes": ["ứng xử", "trung thực"]},
    {"text": "Của một đồng, công một nén", "meaning": "Giá trị của công sức lao động bỏ ra để làm nên một vật thường lớn hơn giá trị vật chất của chính vật đó.", "themes": ["chăm chỉ"]},
    {"text": "Có thực mới vực được đạo", "meaning": "Cần có điều kiện vật chất cơ bản (ăn no) thì con người mới có thể theo đuổi những giá trị tinh thần, đạo lý cao hơn.", "themes": ["kinh nghiệm dân gian"]},
    {"text": "Nhàn cư vi bất thiện", "meaning": "Rảnh rỗi, không có việc làm dễ khiến con người sinh ra những suy nghĩ, hành động không tốt.", "themes": ["chăm chỉ", "đạo đức"]},
    {"text": "Muốn sang thì bắc cầu Kiều, muốn con hay chữ thì yêu lấy thầy", "meaning": "Muốn con cái học giỏi, nên người thì cha mẹ cần tôn trọng, quý mến người thầy dạy dỗ con mình.", "themes": ["học tập", "biết ơn"]},
]