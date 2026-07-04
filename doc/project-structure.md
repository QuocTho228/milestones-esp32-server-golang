Milestones_ESP32_Server_Golang/
├── 📄 go.mod # Module định nghĩa cho Go project
├── 📄 go.sum # Checksum của dependencies
├── 📄 LICENSE # License file
├── 📄 README.md # Tài liệu chính của dự án
├── 📄 .gitignore # Git ignore rules
├── 📄 .dockerignore # Docker ignore rules
├── 📄 .gitmodules # Git submodules configuration
├── .github/ # GitHub workflows & actions
│
├── 📂 ai_doc/ # Tài liệu AI và thiết kế
│ ├── branch_review_findings_20260412.md
│ ├── chat_hook_plugin_v2_design.md
│ ├── chat_session_lazy_reuse_refactor_plan.md
│ ├── knowledge_mcp_auto_trigger_plan.md
│ ├── llm_interrupt_extra_plan.md
│ ├── mqtt_lifecycle_transport_plan.md
│ ├── openclaw_agent_integration_plan.md
│ └── weknora_integration_plan.md
│
├── 📂 cmd/ # Command-line applications (entry points)
│ ├── server/ # Main server application
│ │ ├── main.go # Entry point
│ │ ├── config.go # Server configuration
│ │ ├── asr_server_http.go # ASR HTTP server implementation
│ │ ├── asr_server_http_stub.go # ASR HTTP server stub
│ │ ├── manager_http.go # Manager HTTP implementation
│ │ ├── manager_http_stub.go # Manager HTTP stub
│ │ ├── defaults_asr.go # ASR defaults
│ │ ├── defaults_asr_disabled.go # ASR disabled defaults
│ │ ├── defaults_config_embedded.go
│ │ ├── defaults_config_standard.go
│ │ ├── defaults_manager.go # Manager defaults
│ │ └── defaults_manager_disabled.go
│ ├── mqtt/ # MQTT client application
│ │ └── main.go
│ ├── mock_ai_server/ # Mock AI server for testing
│ │ ├── main.go
│ │ └── main_test.go
│ └── opus_example/ # Opus codec example
│ └── main.go
│
├── 📂 config/ # Configuration files
│ ├── config.yaml # Main configuration
│ ├── mqtt_config.json # MQTT configuration
│ └── models/ # Model configurations
│
├── 📂 constants/ # Constants definitions
│ └── constants.go
│
├── 📂 internal/ # Internal packages (core logic)
│ ├── app/ # Application layer
│ │ ├── server/ # WebSocket/HTTP server
│ │ │ ├── app.go # Server app initialization
│ │ │ ├── event_handle.go # Event handling
│ │ │ ├── message_handle.go # Message handling
│ │ │ ├── auth/ # Authentication
│ │ │ │ ├── auth.go
│ │ │ │ └── auth_test.go
│ │ │ ├── chat/ # Chat/conversation logic
│ │ │ │ ├── asr.go # Speech recognition
│ │ │ │ ├── chat.go # Chat handler
│ │ │ │ ├── common.go
│ │ │ │ ├── llm.go # LLM integration
│ │ │ │ ├── mcp.go # MCP (Model Context Protocol)
│ │ │ │ ├── session.go # Session management
│ │ │ │ ├── speaker.go # Speaker identification
│ │ │ │ ├── tts.go # Text-to-speech
│ │ │ │ ├── tool.go # Tool management
│ │ │ │ ├── vllm.go # vLLM integration
│ │ │ │ ├── media_player.go # Media playback
│ │ │ │ ├── openclaw_warmup.go # OpenClaw warmup
│ │ │ │ ├── realtime_media_gate.go
│ │ │ │ ├── local_mcp_tool.go # Local MCP tools
│ │ │ │ ├── local_mcp_media_control_tool.go
│ │ │ │ ├── mcp_response_types.go
│ │ │ │ ├── mcp_reinit_test.go
│ │ │ │ ├── session_mcp_tool.go
│ │ │ │ ├── types.go
│ │ │ │ ├── util.go
│ │ │ │ ├── plugins/ # Chat plugins
│ │ │ │ ├── [multiple test files]
│ │ │ │
│ │ │ ├── websocket/ # WebSocket server
│ │ │ │ ├── websocket_server.go
│ │ │ │ ├── websocket_conn.go
│ │ │ │ ├── mcp.go # MCP over WebSocket
│ │ │ │ ├── ota.go # OTA (Over-The-Air) updates
│ │ │ │ ├── vision.go # Vision capabilities
│ │ │ │ ├── openclaw.go # OpenClaw integration
│ │ │ │ ├── types.go
│ │ │ │
│ │ │ ├── mqtt_udp/ # MQTT UDP protocol
│ │ │ └── types/ # Type definitions
│ │
│ │ └── mqtt_server/ # MQTT server
│ │ ├── mqtt_server.go
│ │ ├── admin_config.go # Admin configuration
│ │ ├── device_hook.go # Device hooks
│ │ └── auth_hook.go # Authentication hooks
│ │
│ ├── components/ # Reusable components
│ │ └── http/ # HTTP component
│ │ ├── client.go # HTTP client
│ │ ├── manager_client.go # Manager HTTP client
│ │ ├── types.go
│ │ └── README.md
│ │
│ ├── config/ # Configuration management
│ ├── data/ # Data layer
│ ├── db/ # Database layer
│ ├── domain/ # Domain models
│ ├── pkg/ # Public packages
│ ├── pool/ # Object pooling
│ └── util/ # Utility functions
│
├── 📂 logger/ # Logging utilities
│ ├── logger.go
│ └── db_log.go
│
├── 📂 lib/ # External libraries
│ └── ten-vad/ # Voice Activity Detection
│
├── 📂 manager/ # Manager UI/frontend
│ ├── backend/ # Backend (likely Go)
│ └── frontend/ # Frontend (likely React/Vue)
│
├── 📂 build/ # Build configurations
│ ├── common/ # Common build config
│ │ ├── asr_server.json
│ │ ├── manager.json
│ │ ├── main_config.yaml
│ │ ├── models/ # Model files
│ │ └── data/
│ ├── linux/ # Linux build config
│ ├── macos/ # macOS build config
│ │ └── fix_rpath.sh # Fix rpath script
│ └── windows/ # Windows build config
│ ├── start.bat # Start script
│ └── logs/
│
├── 📂 docker/ # Docker configuration
│ ├── Dockerfile # Main Dockerfile
│ ├── Dockerfile.main
│ ├── Dockerfile.backend
│ ├── Dockerfile.frontend
│ ├── Dockerfile.linux
│ ├── Dockerfile.windows
│ ├── Dockerfile_build
│ ├── docker-composer_build.yml
│ ├── docker-composer/ # Docker Compose configs
│ └── lib/ # Docker lib files
│
├── 📂 doc/ # Documentation
│ ├── asr_server_merge_plan.md
│ ├── compile_deploy.md # Compilation & deployment
│ ├── config.md # Configuration guide
│ ├── docker.md, docker_compose.md # Docker documentation
│ ├── esp32_milestones_backend_guide.md # ESP32 guide
│ ├── fullchain_mock_pressure_test_plan.md
│ ├── indextts_vllm_api.md
│ ├── knowledge_base.md
│ ├── manager_console_guide.md
│ ├── mcp.md, mcp_market.md # MCP documentation
│ ├── mcp_audio_example.md
│ ├── mcp_remote_call_agent_device.md
│ ├── mcp_resource.md
│ ├── mock_external_ai_server.md
│ ├── mqtt_bridge.md # MQTT documentation
│ ├── mqtt_udp_protocol.md
│ ├── mqtt_udp.md
│ ├── ota_mqtt_auth.md # OTA documentation
│ ├── openclaw_integration.md # OpenClaw integration
│ ├── quickstart_bundle_tutorial.md
│ ├── speaker_identification.md
│ ├── user_config.md
│ ├── vision.md
│ ├── voice_clone.md
│ ├── websocket_connection_flow.md
│ ├── websocket_meter.md
│ ├── websocket_server.md
│ └── aio/ # AIO documentation
│
└── 📂 test/ # Test files
├── mqtt_password_test.go
├── auto_test/ # Automated tests
├── doubao_asr/ # Doubao ASR tests
├── edge_tts_offline/ # Edge TTS tests
├── interrupt_history/ # Interrupt handling tests
├── mcp/ # MCP tests
├── mcp_client_over_websocket/ # MCP client tests
├── mcp_server_over_websocket/ # MCP server tests
├── mem0/ # Memory tests
├── minimax/ # Minimax tests
├── mqtt_udp/ # MQTT UDP tests
├── music_player/ # Music player tests
├── py_test_audio/ # Python audio tests
├── ten_vad/ # VAD tests
├── test_audio/ # Audio test files
├── test_openclaw_server/ # OpenClaw server tests
├── test_opus/ # Opus codec tests
├── vllm/ # vLLM tests
├── webrtc_vad/ # WebRTC VAD tests
├── websocket_client/ # WebSocket client tests
└── websocket_multi/ # Multi-websocket tests
