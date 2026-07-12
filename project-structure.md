milestones-esp32-server-golang
│ .dockerignore
│ .gitignore
│ .gitmodules
│ .prettierrc
│ go.mod
│ go.sum
│ LICENSE
│ project-structure.md
│ README.md
│
├───.github
│ └───workflows
│ build-release.yml
│ docker-build.yml
│
├───ai_doc
│ branch_review_findings_20260412.md
│ chat_hook_plugin_v2_design.md
│ chat_session_lazy_reuse_refactor_plan.md
│ knowledge_mcp_auto_trigger_plan.md
│ llm_interrupt_extra_plan.md
│ mqtt_lifecycle_transport_plan.md
│ openclaw_agent_integration_plan.md
│ weknora_integration_plan.md
│
├───asr_server
│ │ .gitignore
│ │ config.json
│ │ Dockerfile
│ │ Dockerfile.github
│ │ go.mod
│ │ go.sum
│ │ main.go
│ │ README.md
│ │
│ ├───.github
│ │ └───workflows
│ │ docker-build.yml
│ │
│ ├───config
│ │ config.go
│ │
│ ├───go_test
│ │ └───speaker
│ │ test_speaker.go
│ │
│ ├───internal
│ │ ├───bootstrap
│ │ │ init.go
│ │ │
│ │ ├───config
│ │ │ └───hotreload
│ │ │ hot_reload.go
│ │ │
│ │ ├───handlers
│ │ │ health.go
│ │ │ stats.go
│ │ │
│ │ ├───logger
│ │ │ logger.go
│ │ │
│ │ ├───middleware
│ │ │ rate_limiter.go
│ │ │
│ │ ├───pool
│ │ │ silero_vad_pool.go
│ │ │ ten_vad_cgo.go
│ │ │ ten_vad_pool.go
│ │ │ types.go
│ │ │ vad_factory.go
│ │ │ vad_pool_interface.go
│ │ │
│ │ ├───router
│ │ │ router.go
│ │ │
│ │ ├───session
│ │ │ manager.go
│ │ │
│ │ ├───speaker
│ │ │ handler.go
│ │ │ json_vector_db.go
│ │ │ manager.go
│ │ │ vector_db.go
│ │ │
│ │ └───ws
│ │ websocket.go
│ │
│ ├───lib
│ │ │ libonnxruntime.so
│ │ │ libsherpa-onnx-c-api.so
│ │ │
│ │ └───ten-vad
│ │ ├───include
│ │ │ ten_vad.h
│ │ │ ten_vad.py
│ │ │
│ │ └───lib
│ │ ├───Linux
│ │ │ └───x64
│ │ │ libten_vad.so
│ │ │
│ │ ├───macOS
│ │ │ └───ten_vad.framework
│ │ │ │ Headers
│ │ │ │ Resources
│ │ │ │ ten_vad
│ │ │ │
│ │ │ └───Versions
│ │ │ │ Current
│ │ │ │
│ │ │ └───A
│ │ │ │ ten_vad
│ │ │ │
│ │ │ ├───Headers
│ │ │ │ ten_vad.h
│ │ │ │
│ │ │ └───Resources
│ │ │ Info.plist
│ │ │
│ │ └───Windows
│ │ ├───x64
│ │ │ ten_vad.dll
│ │ │ ten_vad.lib
│ │ │
│ │ └───x86
│ │ ten_vad.dll
│ │ ten_vad.lib
│ │
│ ├───models
│ │ └───speaker
│ │ 3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx
│ │
│ ├───scripts
│ │ generate_ssl_certs.go
│ │ nginx.conf
│ │
│ ├───server
│ │ setup.go
│ │
│ ├───static
│ │ index.html
│ │
│ └───test
│ ├───asr
│ │ │ audiofile_test.py
│ │ │ stress_test.py
│ │ │
│ │ └───test_wavs
│ │ en.wav
│ │ yue.wav
│ │ zh.wav
│ │
│ └───spearker
│ test_speaker_api.py
│ test_web_interface.py
│
├───build
│ ├───common
│ │ │ asr_server.json
│ │ │ main_config.yaml
│ │ │ manager.json
│ │ │
│ │ ├───data
│ │ │ └───speaker
│ │ │ speaker_embeddings.json
│ │ │
│ │ └───models
│ │ ├───speaker
│ │ │ 3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx
│ │ │
│ │ └───vad
│ │ silero_vad.onnx
│ │
│ ├───linux
│ │ libten_vad.so
│ │
│ ├───macos
│ │ fix_rpath.sh
│ │
│ └───windows
│ │ start.bat
│ │ ten_vad.dll
│ │
│ └───logs
│ server.log.20260204
│
├───cmd
│ ├───mock_ai_server
│ │ main.go
│ │ main_test.go
│ │
│ ├───mqtt
│ │ main.go
│ │
│ ├───opus_example
│ │ main.go
│ │
│ └───server
│ asr_server_http.go
│ asr_server_http_stub.go
│ config.go
│ defaults_asr.go
│ defaults_asr_disabled.go
│ defaults_config_embedded.go
│ defaults_config_standard.go
│ defaults_manager.go
│ defaults_manager_disabled.go
│ main.go
│ manager_http.go
│ manager_http_stub.go
│
├───config
│ │ config.yaml
│ │ mqtt_config.json
│ │
│ └───models
│ └───vad
│ silero_vad.onnx
│
├───constants
│ constants.go
│
├───doc
│ │ asr_server_merge_plan.md
│ │ compile_deploy.md
│ │ config.md
│ │ delay_test.md
│ │ docker.md
│ │ docker_compose.md
│ │ docker_deployment.md
│ │ esp32_milestones_backend_guide.md
│ │ fullchain_mock_pressure_test_plan.md
│ │ indextts_vllm_api.md
│ │ knowledge_base.md
│ │ manager_console_guide.md
│ │ mcp.md
│ │ mcp_audio_example.md
│ │ mcp_market.md
│ │ mcp_remote_call_agent_device.md
│ │ mcp_resource.md
│ │ mock_external_ai_server.md
│ │ mqtt_bridge.md
│ │ mqtt_udp.md
│ │ mqtt_udp_protocol.md
│ │ openclaw_integration.md
│ │ ota_mqtt_auth.md
│ │ project-structure.md
│ │ quickstart_bundle_tutorial.md
│ │ speaker_identification.md
│ │ user_config.md
│ │ vision.md
│ │ voice_clone.md
│ │ websocket_connection_flow.md
│ │ websocket_meter.md
│ │ websocket_server.md
│ │
│ └───aio
│ README_linux.md
│ README_macos.md
│ README_windows.md
│
├───docker
│ │ docker-composer_build.yml
│ │ Dockerfile
│ │ Dockerfile.backend
│ │ Dockerfile.frontend
│ │ Dockerfile.linux
│ │ Dockerfile.main
│ │ Dockerfile.mcp
│ │ Dockerfile.windows
│ │ Dockerfile_build
│ │
│ ├───docker-composer
│ │ docker-compose.local.yml
│ │ docker-compose.yml
│ │
│ └───lib
│ onnxruntime-linux-x64-1.21.0.tgz
│
├───internal
│ ├───app
│ │ ├───mqtt_server
│ │ │ admin_config.go
│ │ │ auth_hook.go
│ │ │ device_hook.go
│ │ │ mqtt_server.go
│ │ │
│ │ └───server
│ │ │ app.go
│ │ │ event_handle.go
│ │ │ message_handle.go
│ │ │
│ │ ├───auth
│ │ │ auth.go
│ │ │ auth_test.go
│ │ │
│ │ ├───chat
│ │ │ │ asr.go
│ │ │ │ asr_history_audio_test.go
│ │ │ │ chat.go
│ │ │ │ chat_goodbye_retention_test.go
│ │ │ │ chat_session_close_test.go
│ │ │ │ common.go
│ │ │ │ llm.go
│ │ │ │ llm_test.go
│ │ │ │ local_mcp_media_control_tool.go
│ │ │ │ local_mcp_tool.go
│ │ │ │ mcp.go
│ │ │ │ mcp_reinit_test.go
│ │ │ │ mcp_response_types.go
│ │ │ │ media_player.go
│ │ │ │ media_player_test.go
│ │ │ │ openclaw_warmup.go
│ │ │ │ openclaw_warmup_test.go
│ │ │ │ realtime_media_gate.go
│ │ │ │ realtime_media_gate_test.go
│ │ │ │ server_transport.go
│ │ │ │ session.go
│ │ │ │ session_abort_test.go
│ │ │ │ session_detect_test.go
│ │ │ │ session_mcp_tool.go
│ │ │ │ speaker.go
│ │ │ │ speak_request_test.go
│ │ │ │ tool.go
│ │ │ │ tool_test.go
│ │ │ │ tts.go
│ │ │ │ tts_test.go
│ │ │ │ types.go
│ │ │ │ util.go
│ │ │ │ vllm.go
│ │ │ │
│ │ │ └───plugins
│ │ │ base.go
│ │ │ output_segmenter.go
│ │ │ output_segmenter_test.go
│ │ │
│ │ ├───mqtt_udp
│ │ │ mqtt_udp_adapter.go
│ │ │ mqtt_udp_conn.go
│ │ │ mqtt_udp_lifecycle_test.go
│ │ │ mqtt_udp_session_test.go
│ │ │ udp.go
│ │ │ udp_server.go
│ │ │
│ │ ├───types
│ │ │ conn.go
│ │ │
│ │ └───websocket
│ │ mcp.go
│ │ openclaw.go
│ │ ota.go
│ │ types.go
│ │ vision.go
│ │ websocket_conn.go
│ │ websocket_server.go
│ │
│ ├───components
│ │ └───http
│ │ client.go
│ │ manager_client.go
│ │ README.md
│ │ types.go
│ │
│ ├───config
│ │ config.go
│ │
│ ├───data
│ │ ├───audio
│ │ │ audio.go
│ │ │
│ │ ├───client
│ │ │ asr.go
│ │ │ asr_test.go
│ │ │ audio_idle.go
│ │ │ audio_idle_test.go
│ │ │ client.go
│ │ │ idle_timeout_test.go
│ │ │ mqtt.go
│ │ │ statistics.go
│ │ │ vad.go
│ │ │ voice_status.go
│ │ │
│ │ ├───history
│ │ │ client.go
│ │ │
│ │ └───msg
│ │ message_types.go
│ │
│ ├───db
│ │ └───redis
│ │ redis.go
│ │
│ ├───domain
│ │ │ message_types.go
│ │ │
│ │ ├───asr
│ │ │ │ adapter.go
│ │ │ │ aliyun_funasr_adapter.go
│ │ │ │ aliyun_qwen3_adapter.go
│ │ │ │ base.go
│ │ │ │ xunfei_adapter.go
│ │ │ │
│ │ │ ├───aliyun_funasr
│ │ │ │ config.go
│ │ │ │ config_test.go
│ │ │ │ engine.go
│ │ │ │ engine_test.go
│ │ │ │ protocol.go
│ │ │ │
│ │ │ ├───aliyun_qwen3
│ │ │ │ config.go
│ │ │ │ engine.go
│ │ │ │ protocol.go
│ │ │ │ protocol_test.go
│ │ │ │
│ │ │ ├───doubao
│ │ │ │ │ adapter.go
│ │ │ │ │ doubao.go
│ │ │ │ │ types.go
│ │ │ │ │
│ │ │ │ ├───client
│ │ │ │ │ client_stream.go
│ │ │ │ │
│ │ │ │ ├───common
│ │ │ │ │ common.go
│ │ │ │ │
│ │ │ │ ├───request
│ │ │ │ │ header.go
│ │ │ │ │ payload.go
│ │ │ │ │
│ │ │ │ └───response
│ │ │ │ response.go
│ │ │ │
│ │ │ ├───funasr
│ │ │ │ │ funasr.go
│ │ │ │ │
│ │ │ │ └───example
│ │ │ │ streaming_example.go
│ │ │ │
│ │ │ ├───types
│ │ │ │ types.go
│ │ │ │
│ │ │ └───xunfei
│ │ │ config.go
│ │ │ engine.go
│ │ │ protocol.go
│ │ │
│ │ ├───audio
│ │ │ audio_handler.go
│ │ │
│ │ ├───chat
│ │ │ ├───hooks
│ │ │ │ hub.go
│ │ │ │ statistic_plugin.go
│ │ │ │ statistic_plugin_test.go
│ │ │ │ types.go
│ │ │ │
│ │ │ └───streamtransform
│ │ │ pipeline.go
│ │ │ pipeline_test.go
│ │ │ types.go
│ │ │
│ │ ├───config
│ │ │ │ base.go
│ │ │ │ config_init.go
│ │ │ │ interface.go
│ │ │ │ provider_test.go
│ │ │ │
│ │ │ ├───manager
│ │ │ │ auth.go
│ │ │ │ configtest.go
│ │ │ │ configtest_test.go
│ │ │ │ manager.go
│ │ │ │ manager_test.go
│ │ │ │ README.md
│ │ │ │ types.go
│ │ │ │ websocket_client.go
│ │ │ │
│ │ │ ├───memory
│ │ │ │ memory.go
│ │ │ │
│ │ │ ├───redis
│ │ │ │ auth.go
│ │ │ │ llm_memory.go
│ │ │ │ userconfig.go
│ │ │ │
│ │ │ └───types
│ │ │ activition.go
│ │ │ event.go
│ │ │ types.go
│ │ │
│ │ ├───doubaoapi
│ │ │ headers.go
│ │ │
│ │ ├───eventbus
│ │ │ add_message_types.go
│ │ │ chat_history_types.go
│ │ │ eventbus.go
│ │ │ exit_chat_types.go
│ │ │ types.go
│ │ │
│ │ ├───llm
│ │ │ │ base.go
│ │ │ │ llm.go
│ │ │ │
│ │ │ ├───common
│ │ │ │ helpers.go
│ │ │ │ types.go
│ │ │ │
│ │ │ ├───coze_llm
│ │ │ │ coze_llm.go
│ │ │ │
│ │ │ ├───dify_llm
│ │ │ │ dify_llm.go
│ │ │ │
│ │ │ ├───eino_llm
│ │ │ │ eino_llm.go
│ │ │ │ eino_llm_test.go
│ │ │ │ eino_vllm.go
│ │ │ │ example.go
│ │ │ │ README.md
│ │ │ │ REFACTOR_SUMMARY.md
│ │ │ │ thinking.go
│ │ │ │
│ │ │ └───test
│ │ │ llm_test.go
│ │ │ splite_content.go
│ │ │
│ │ ├───mcp
│ │ │ config_checker.go
│ │ │ device_manager.go
│ │ │ global_manage.go
│ │ │ iot_over_mcp_transport.go
│ │ │ local_manager.go
│ │ │ manager.go
│ │ │ mcp_client.go
│ │ │ mcp_pool.go
│ │ │ mcp_test.go
│ │ │ mcp_tool.go
│ │ │ README.md
│ │ │ SSE_REFACTOR_SUMMARY.md
│ │ │ tool_name.go
│ │ │ types.go
│ │ │ validator.go
│ │ │ websocket_transport.go
│ │ │
│ │ ├───memory
│ │ │ │ base.go
│ │ │ │
│ │ │ ├───llm_memory
│ │ │ │ llm_memory.go
│ │ │ │ types.go
│ │ │ │
│ │ │ ├───mem0
│ │ │ │ mem0_client.go
│ │ │ │
│ │ │ ├───memobase
│ │ │ │ memobase_client.go
│ │ │ │
│ │ │ ├───memos
│ │ │ │ API_DESIGN.md
│ │ │ │ memos_client.go
│ │ │ │ memos_client_test.go
│ │ │ │
│ │ │ └───nomemo
│ │ │ nomemo.go
│ │ │
│ │ ├───openclaw
│ │ │ manager.go
│ │ │ manager_test.go
│ │ │
│ │ ├───play_music
│ │ │ music_player.go
│ │ │ README.md
│ │ │ types.go
│ │ │
│ │ ├───rag
│ │ │ dify_searcher.go
│ │ │ interface.go
│ │ │ manager.go
│ │ │ ragflow_searcher.go
│ │ │ weknora_searcher.go
│ │ │
│ │ ├───speaker
│ │ │ asr_server.go
│ │ │ base.go
│ │ │ streaming.go
│ │ │ types.go
│ │ │
│ │ ├───tts
│ │ │ │ base.go
│ │ │ │ base_test.go
│ │ │ │
│ │ │ ├───cosyvoice
│ │ │ │ cosyvoice.go
│ │ │ │ cosyvoice_test.go
│ │ │ │
│ │ │ ├───doubao
│ │ │ │ doubao.go
│ │ │ │ doubao_test.go
│ │ │ │ doubao_ws.go
│ │ │ │ doubao_ws_test.go
│ │ │ │ model.go
│ │ │ │
│ │ │ ├───edge
│ │ │ │ edge.go
│ │ │ │ edge_test.go
│ │ │ │
│ │ │ ├───edge_offline
│ │ │ │ edge_offline.go
│ │ │ │ edge_offline_test.go
│ │ │ │
│ │ │ ├───indextts_vllm
│ │ │ │ indextts_vllm.go
│ │ │ │ indextts_vllm_test.go
│ │ │ │
│ │ │ ├───milestones
│ │ │ │ milestones.go
│ │ │ │ milestones_test.go
│ │ │ │
│ │ │ ├───minimax
│ │ │ │ minimax.go
│ │ │ │ minimax_test.go
│ │ │ │
│ │ │ ├───openai
│ │ │ │ openai.go
│ │ │ │ openai_test.go
│ │ │ │
│ │ │ ├───qwen
│ │ │ │ qwen_tts.go
│ │ │ │ qwen_tts_test.go
│ │ │ │
│ │ │ ├───streaming
│ │ │ │ events.go
│ │ │ │
│ │ │ ├───xunfei
│ │ │ │ xunfei.go
│ │ │ │ xunfei_test.go
│ │ │ │
│ │ │ ├───xunfei_super_tts
│ │ │ │ xunfei_super_tts.go
│ │ │ │ xunfei_super_tts_test.go
│ │ │ │
│ │ │ └───zhipu
│ │ │ zhipu.go
│ │ │ zhipu_test.go
│ │ │
│ │ └───vad
│ │ │ base.go
│ │ │
│ │ ├───inter
│ │ │ vad.go
│ │ │
│ │ ├───silero_vad
│ │ │ vad.go
│ │ │ vad_test.go
│ │ │
│ │ ├───ten_vad
│ │ │ ten_vad_cgo.go
│ │ │ vad.go
│ │ │
│ │ ├───test
│ │ │ silero_vad.onnx
│ │ │ vad
│ │ │ vad.go
│ │ │ wav2vad.go
│ │ │
│ │ └───webrtc_vad
│ │ factor.go
│ │ README.md
│ │ util.go
│ │ webrtc_vad.go
│ │ webrtc_vad_pool_test.go
│ │ webrtc_vad_test.go
│ │
│ ├───pkg
│ │ └───hooks
│ │ hub.go
│ │ hub_test.go
│ │
│ ├───pool
│ │ adapter.go
│ │ manager.go
│ │ reporter.go
│ │
│ └───util
│ │ audio_utils.go
│ │ audio_utils_test.go
│ │ backend_url.go
│ │ buffer.go
│ │ encryption.go
│ │ manager_auth.go
│ │ ogg_opus.go
│ │ opus_repacketizer.go
│ │ password_signature.go
│ │ queue.go
│ │ queue_test.go
│ │ resource_pool.go
│ │ sentence.go
│ │ sentence_test.go
│ │ voice.go
│ │
│ └───workqueue
│ parallelizer.go
│ parallelizer_test.go
│ parallelizer_test.go.bak
│
├───lib
│ └───ten-vad
│ ├───include
│ │ ten_vad.h
│ │
│ └───lib
│ ├───Linux
│ │ └───x64
│ │ libten_vad.so
│ │
│ ├───macOS
│ │ └───ten_vad.framework
│ │ │ Headers
│ │ │ Resources
│ │ │ ten_vad
│ │ │
│ │ └───Versions
│ │ │ Current
│ │ │
│ │ └───A
│ │ │ ten_vad
│ │ │
│ │ ├───Headers
│ │ │ ten_vad.h
│ │ │
│ │ └───Resources
│ │ Info.plist
│ │
│ └───Windows
│ └───x64
│ ten_vad.dll
│ ten_vad.lib
│
├───logger
│ db_log.go
│ logger.go
│
├───logs
│ server.log
│ server.log.20260709
│ server.log.20260710
│ server.log.20260711
│
├───manager
│ ├───backend
│ │ │ Dockerfile
│ │ │ go.mod
│ │ │ go.sum
│ │ │ main.go
│ │ │ start.bat
│ │ │ start.sh
│ │ │
│ │ ├───config
│ │ │ config.go
│ │ │ config.json
│ │ │ internal_auth.go
│ │ │ README.md
│ │ │
│ │ ├───controllers
│ │ │ admin.go
│ │ │ admin_mcp_endpoint_test.go
│ │ │ agent_device_service.go
│ │ │ agent_device_service_test.go
│ │ │ agent_mcp_services.go
│ │ │ api_token.go
│ │ │ auth.go
│ │ │ chat_history.go
│ │ │ common.go
│ │ │ device_activation.go
│ │ │ device_update_helper.go
│ │ │ doubao_model.go
│ │ │ doubao_voice_clone.go
│ │ │ knowledge.go
│ │ │ knowledge_sync.go
│ │ │ knowledge_sync_async.go
│ │ │ mcp_config_tools.go
│ │ │ mcp_market.go
│ │ │ mcp_market_imported_services.go
│ │ │ openclaw_helpers.go
│ │ │ openclaw_sse.go
│ │ │ pool_stats.go
│ │ │ runtime_info.go
│ │ │ setup.go
│ │ │ simple_captcha.go
│ │ │ speaker_group.go
│ │ │ system_configs_test.go
│ │ │ user.go
│ │ │ voices.go
│ │ │ voice_clone.go
│ │ │ voice_clone_task_worker.go
│ │ │ voice_clone_test.go
│ │ │ voice_constants.go
│ │ │ websocket.go
│ │ │
│ │ ├───database
│ │ │ database.go
│ │ │ database_reset.go
│ │ │ database_test.go
│ │ │
│ │ ├───middleware
│ │ │ auth.go
│ │ │ internal_auth.go
│ │ │ openapi_auth.go
│ │ │
│ │ ├───models
│ │ │ models.go
│ │ │
│ │ ├───router
│ │ │ router.go
│ │ │
│ │ ├───services
│ │ │ ├───configprovider
│ │ │ │ provider.go
│ │ │ │ provider_test.go
│ │ │ │
│ │ │ └───mcp_market
│ │ │ catalog.go
│ │ │ crypto.go
│ │ │ providers.go
│ │ │ types.go
│ │ │
│ │ ├───static
│ │ │ embed_ui.go
│ │ │ stub.go
│ │ │
│ │ └───storage
│ │ │ audio.go
│ │ │ config_adapter.go
│ │ │ factory.go
│ │ │ gorm_agent.go
│ │ │ gorm_base.go
│ │ │ gorm_config.go
│ │ │ gorm_device.go
│ │ │ gorm_user.go
│ │ │ interfaces.go
│ │ │ pool_stats.go
│ │ │ storage_adapter.go
│ │ │
│ │ ├───mysql
│ │ │ config.go
│ │ │ storage.go
│ │ │
│ │ └───sqlite
│ │ config.go
│ │ storage.go
│ │
│ └───frontend
│ │ diagnose.js
│ │ Dockerfile
│ │ index.html
│ │ nginx.conf
│ │ package-lock.json
│ │ package.json
│ │ vite.config.js
│ │
│ ├───public
│ │ apple-light-preview.html
│ │ apple-touch-icon.png
│ │ diagnose.js
│ │ favicon.png
│ │
│ └───src
│ │ App.vue
│ │ main.js
│ │
│ ├───assets
│ │ ├───agent-status-icons
│ │ │ knowledge-base.png
│ │ │ mcp.png
│ │ │ memory.png
│ │ │ openclaw.png
│ │ │
│ │ └───brand
│ │ app-logo.webp
│ │
│ ├───components
│ │ │ AppHeader.vue
│ │ │ Layout.vue
│ │ │ MobileLayout.vue
│ │ │ MobileNavBar.vue
│ │ │ MobileTabBar.vue
│ │ │
│ │ ├───common
│ │ │ AgentForm.vue
│ │ │ AgentRuntimeDiagnostics.vue
│ │ │ DeviceForm.vue
│ │ │
│ │ └───user
│ │ MessageInjectDialog.vue
│ │
│ ├───composables
│ │ useAgentFormOptions.js
│ │
│ ├───router
│ │ index.js
│ │
│ ├───stores
│ │ auth.js
│ │
│ ├───styles
│ │ apple-light.css
│ │
│ ├───utils
│ │ api.js
│ │ authRedirect.js
│ │ configTest.js
│ │ device.js
│ │ openclaw.js
│ │ setupStatus.js
│ │ sse.js
│ │
│ └───views
│ │ Dashboard.vue
│ │ Login.vue
│ │ OpenAPIDocs.vue
│ │ Setup.vue
│ │ SimpleLogin.vue
│ │ Test.vue
│ │ TestRoute.vue
│ │
│ ├───admin
│ │ │ Agents.vue
│ │ │ ASRConfig.vue
│ │ │ ChatSettings.vue
│ │ │ ConfigWizard.vue
│ │ │ Devices.vue
│ │ │ GlobalRoles.vue
│ │ │ KnowledgeSearchConfig.vue
│ │ │ LLMConfig.vue
│ │ │ MCPConfig.vue
│ │ │ MCPMarket.vue
│ │ │ MemoryConfig.vue
│ │ │ MQTTConfig.vue
│ │ │ MQTTServerConfig.vue
│ │ │ OTAConfig.vue
│ │ │ PoolStats.vue
│ │ │ SpeakerConfig.vue
│ │ │ TTSConfig.vue
│ │ │ UDPConfig.vue
│ │ │ Users.vue
│ │ │ VADConfig.vue
│ │ │ VisionConfig.vue
│ │ │
│ │ └───forms
│ │ ASRConfigForm.vue
│ │ configProviderUtils.js
│ │ llmCatalog.js
│ │ LLMConfigForm.vue
│ │ TTSConfigForm.vue
│ │ ttsProviderOptions.js
│ │ VADConfigForm.vue
│ │ XunfeiCommonConfig.vue
│ │
│ ├───mobile
│ │ MobileLogin.vue
│ │ MobileMore.vue
│ │
│ └───user
│ AgentDevices.vue
│ AgentEdit.vue
│ AgentHistory.vue
│ Agents.vue
│ APITokens.vue
│ KnowledgeBases.vue
│ Roles.vue
│ Speakers.vue
│ VoiceClones.vue
│
└───test
│ mqtt_password_test.go
│
├───auto_test
│ agent_ws_endpoint_mcp.go
│ audio_utils.go
│ automation_suite.go
│ mcp_server.go
│ mcp_transport.go
│ milestones_websocket_client.go
│ mqtt_udp_full_suite.go
│ ota_mqtt_udp_suite.go
│ TEST_CASE_COVERAGE.md
│ test_mp3.go
│ vllm.go
│
├───doubao_asr
│ main.go
│
├───edge_tts_offline
│ main.go
│
├───interrupt_history
│ main.go
│
├───mcp
│ mcp_client.go
│
├───mcp_client_over_websocket
│ main.go
│ ws_transport.go
│
├───mcp_server_over_websocket
│ mcp_websocket.go
│ websocket_server.go
│
├───mem0
│ │ memory.py
│ │
│ └───**pycache**
│ mem0.cpython-312.pyc
│
├───minimax
│ main.go
│
├───mqtt_udp
│ audio_utils
│ audio_utils.go
│ go.sum
│ main.go
│ mqtt.go
│ mqtt.py
│ ota.go
│ README.md
│ test_24000.wav
│ udp.go
│
├───music_player
│ main.go
│
├───py_test_audio
│ dec_opus.py
│ main.py
│
├───ten_vad
│ audio_utils.go
│ main.go
│ README.md
│
├───test_audio
│ audio_utils.go
│ main.go
│
├───test_openclaw_server
│ main.go
│ README.md
│
├───test_opus
│ decode_opus.c
│
├───vllm
│ main.go
│
├───webrtc_vad
│ audio_utils.go
│ main.go
│
├───websocket_client
│ audio_utils.go
│ mcp_server.go
│ mcp_transport.go
│ milestones_websocket_client.go
│ test.mp3
│ test_mp3.go
│ vllm.go
│
└───websocket_multi
audio_utils.go
milestones_ws_client_multi.go
README.md
summarize_metrics.py
