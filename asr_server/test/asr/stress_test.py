#!/usr/bin/env python3
"""
VAD ASR 服务器压力测试工具（优化版）
支持多并发连接，每个连接发送多个音频文件
实时监控系统资源使用情况
详细统计测试结果并输出报告
"""

import asyncio
import websockets
import time
import threading
import random
import os
import sys
import wave
import struct
import statistics
import psutil  # 用于资源监控
from collections import deque
from datetime import datetime
import platform
import argparse
import aiohttp  # 用于HTTP健康检查
import json
# 设置信号处理
import signal
def signal_handler(signum, frame):
    print("\n🛑 收到中断信号，正在停止测试...")
    sys.exit(1)

# 定义性能指标数据结构
class PerformanceMetrics:
    def __init__(self):
        self.start_time = None
        self.end_time = None
        self.total_connections = 0
        self.successful_connections = 0
        self.total_audio_files = 0
        self.successful_recognitions = 0
        self.recognition_results = []
        self.response_times = []
        self.errors = []
        self.connection_times = []
        self.system_stats = []  # 存储系统资源使用情况
        self.connection_success_rates = []
        
        # 实时监控队列
        self.cpu_usage = deque(maxlen=100)
        self.memory_usage = deque(maxlen=100)
        self.network_io = deque(maxlen=100)
        
        # 测试配置
        self.config = {}
        
    def add_system_stat(self, cpu, memory, net_io):
        """记录系统资源使用情况"""
        timestamp = time.time()
        self.system_stats.append({
            "timestamp": timestamp,
            "cpu": cpu,
            "memory": memory,
            "network": net_io
        })
        self.cpu_usage.append(cpu)
        self.memory_usage.append(memory)
        self.network_io.append(net_io)
    
    def get_summary(self):
        """获取摘要统计信息"""
        return {
            "total_time": self.end_time - self.start_time if self.end_time else 0,
            "total_connections": self.total_connections,
            "connection_success_rate": self.successful_connections / self.total_connections if self.total_connections else 0,
            "audio_files_per_sec": self.total_audio_files / (self.end_time - self.start_time) if self.end_time and self.end_time > self.start_time else 0,
            "recognitions_per_sec": self.successful_recognitions / (self.end_time - self.start_time) if self.end_time and self.end_time > self.start_time else 0,
            "avg_response_time": statistics.mean(self.response_times) if self.response_times else 0,
            "min_response_time": min(self.response_times) if self.response_times else 0,
            "max_response_time": max(self.response_times) if self.response_times else 0,
            "avg_cpu_usage": statistics.mean(self.cpu_usage) if self.cpu_usage else 0,
            "max_cpu_usage": max(self.cpu_usage) if self.cpu_usage else 0,
            "avg_memory_usage": statistics.mean(self.memory_usage) if self.memory_usage else 0,
            "max_memory_usage": max(self.memory_usage) if self.memory_usage else 0,
            "avg_network_io": statistics.mean([io[0] for io in self.network_io]) if self.network_io else 0,
        }

# 全局性能指标
metrics = PerformanceMetrics()

def get_audio_files(directory="test_wavs"):
    """获取测试音频文件列表，并验证音频格式"""
    audio_files = []
    if not os.path.exists(directory) or not os.path.isdir(directory):
        print(f"❌ 错误：目录不存在: {directory}")
        return []
    
    # 动态读取目录下所有wav文件
    import glob
    wav_pattern = os.path.join(directory, "*.wav")
    wav_paths = glob.glob(wav_pattern)
    
    if not wav_paths:
        print(f"❌ 错误：目录中没有找到wav文件: {directory}")
        return []
    
    audio_files = sorted(wav_paths)  # 排序以保证测试的一致性
    
    # 验证音频格式
    valid_audio_files = []
    for file_path in audio_files:
        try:
            with wave.open(file_path, 'rb') as wf:
                channels = wf.getnchannels()
                sample_width = wf.getsampwidth()
                sample_rate = wf.getframerate()
                frames = wf.getnframes()
                duration = frames / sample_rate
                
                if channels != 1:
                    print(f"⚠️  警告：文件不是单声道 ({os.path.basename(file_path)} 有 {channels} 个声道), 将进行转换")
                if sample_width != 2:
                    print(f"⚠️  警告：文件不是16位PCM ({os.path.basename(file_path)} 是 {sample_width*8} 位), 可能影响识别")
                if duration > 60:
                    print(f"⚠️  警告：文件过长 ({duration:.1f}s), 可能影响性能")
                    
                valid_audio_files.append(file_path)
        except Exception as e:
            print(f"❌ 错误：无法验证音频文件 {file_path}: {e}")
    
    if not valid_audio_files:
        print("❌ 错误：没有有效的音频文件")
        return []
    
    print(f"✅ 找到 {len(valid_audio_files)} 个有效的音频文件")
    return valid_audio_files

def read_wav_file(file_path):
    """
    读取WAV文件并返回音频数据
    如果是立体声，转换为单声道（取左声道）
    """
    try:
        with wave.open(file_path, 'rb') as wav_file:
            # 获取音频参数
            sample_rate = wav_file.getframerate()
            channels = wav_file.getnchannels()
            sample_width = wav_file.getsampwidth()
            frames = wav_file.getnframes()
            audio_data = wav_file.readframes(frames)
            
            # 立体声转单声道
            if channels == 2:
                # 16位立体声: 每个采样有两个16位值（左声道和右声道）
                if sample_width == 2:
                    # 解包成short数组 (每个采样2字节)
                    unpacked_data = struct.unpack(f'<{len(audio_data)//2}h', audio_data)
                    # 取左声道 (偶数索引)
                    mono_data = unpacked_data[0::2]
                    # 重新打包成字节数据
                    audio_data = struct.pack(f'<{len(mono_data)}h', *mono_data)
                else:
                    # 暂时不支持非16位的立体声转换
                    print(f"⚠️  警告：不支持 {sample_width} 字节/样本的立体声转换，将跳过处理")
            
            duration = len(audio_data) / (sample_rate * sample_width * 1)  # 单声道
            
            return {
                "data": audio_data,
                "sample_rate": sample_rate,
                "channels": 1,  # 现在是单声道
                "sample_width": sample_width,
                "duration": duration
            }
    except Exception as e:
        print(f"❌ 错误：读取音频文件失败 {file_path}: {e}")
        return None

async def system_monitor(interval=2):
    """系统资源监控任务"""
    prev_net_io = psutil.net_io_counters().bytes_sent + psutil.net_io_counters().bytes_recv
    
    while True:
        # CPU使用率
        cpu_percent = psutil.cpu_percent(interval=None)
        
        # 内存使用率
        mem = psutil.virtual_memory()
        mem_percent = mem.percent
        
        # 网络IO
        net_io = psutil.net_io_counters()
        current_net_io = net_io.bytes_sent + net_io.bytes_recv
        net_io_diff = current_net_io - prev_net_io
        prev_net_io = current_net_io
        
        # 添加系统指标
        metrics.add_system_stat(cpu_percent, mem_percent, net_io_diff)
        
        await asyncio.sleep(interval)

async def send_audio(websocket, connection_id, audio_info, audio_index):
    """发送单个音频文件"""
    audio_data = audio_info["data"]
    sample_rate = audio_info["sample_rate"]
    sample_width = audio_info["sample_width"]
    duration = audio_info["duration"]
    
    start_time = time.time()
    recognition_success = False
    recognition_result = None
    error_msg = None
    timeout_occurred = False
    
    try:
        # 设置接收超时（基于音频时长，至少10秒，最多60秒）
        receive_timeout = max(min(duration * 2.0 + 5.0, 60.0), 10.0)
        result_event = asyncio.Event()
        
        # 接收消息的异步任务
        async def receive_messages():
            nonlocal recognition_success, recognition_result, error_msg
            try:
                while True:
                    # 设置接收超时
                    response = await asyncio.wait_for(websocket.recv(), timeout=receive_timeout)
                    
                    try:
                        data = json.loads(response)
                        msg_type = data.get('type', 'unknown')
                        
                        if msg_type == 'final':
                            text = data.get('text', '')
                            if text:
                                recognition_result = text
                                recognition_success = True
                                # 实时打印识别结果
                                print(f"\n🎯 连接{connection_id} 音频{audio_index} 识别结果: {text}")
                                result_event.set()
                                return
                        elif msg_type == 'error':
                            error_msg = data.get('message', '未知错误')
                            result_event.set()
                            return
                    except json.JSONDecodeError:
                        pass
            except asyncio.TimeoutError:
                pass
            except Exception as e:
                error_msg = f"接收异常: {e}"
                result_event.set()
        
        # 启动接收任务
        receive_task = asyncio.create_task(receive_messages())
        
        # 分块发送音频数据
        chunk_size = 8192  # 8KB
        total_size = len(audio_data)
        num_chunks = (total_size + chunk_size - 1) // chunk_size
        
        # 计算块间隔时间（模拟实时）
        time_per_byte = 1.0 / (sample_rate * sample_width)
        chunk_interval = chunk_size * time_per_byte
        
        # 发送音频块
        for pos in range(0, total_size, chunk_size):
            chunk = audio_data[pos:pos+chunk_size]
            await websocket.send(chunk)
            # 控制发送速率（模拟实时）
            await asyncio.sleep(chunk_interval)
        
        # 等待结果
        try:
            await asyncio.wait_for(result_event.wait(), timeout=receive_timeout)
        except asyncio.TimeoutError:
            timeout_occurred = True
        
        # 取消接收任务
        receive_task.cancel()
        try:
            await receive_task
        except asyncio.CancelledError:
            pass
            
    except Exception as e:
        error_msg = f"发送异常: {e}"
    
    # 计算响应时间
    response_time = time.time() - start_time
    
    return recognition_success, error_msg, response_time, recognition_result, timeout_occurred

async def test_connection(connection_id, audio_files, results):
    """测试单个连接"""
    connection_results = {
        "id": connection_id,
        "start_time": time.time(),
        "end_time": None,
        "success": False,
        "audio_tests": [],
        "errors": []
    }
    
    try:
        # 连接超时设置
        connect_timeout = 30.0  # 30秒连接超时
        
        # 建立WebSocket连接
        websocket = await asyncio.wait_for(
            websockets.connect(results.config["server_url"], open_timeout=connect_timeout),
            timeout=connect_timeout
        )
        
        # 更新状态
        connection_results["success"] = True
        connection_results["connected"] = time.time()
        
        # 测试每个音频文件
        for idx, audio_file in enumerate(audio_files):
            # 读取音频文件
            audio_info = read_wav_file(audio_file)
            if not audio_info:
                error_msg = f"无法读取音频文件: {audio_file}"
                connection_results["errors"].append(error_msg)
                continue
            
            # 执行音频测试
            test_start = time.time()
            success, error, response_time, result, timeout = await send_audio(
                websocket, connection_id, audio_info, idx+1
            )
            test_duration = time.time() - test_start
            
            # 记录结果
            test_result = {
                "audio_file": os.path.basename(audio_file),
                "success": success,
                "timeout": timeout,
                "error": error,
                "result": result,
                "response_time": response_time,
                "duration": test_duration,
            }
            connection_results["audio_tests"].append(test_result)
            
            # 打印测试结果
            if success:
                print(f"✅ 连接{connection_id} - {os.path.basename(audio_file)}: 识别成功 ({response_time:.2f}s)")
                print(f"   📝 结果: '{result}'")
            else:
                print(f"❌ 连接{connection_id} - {os.path.basename(audio_file)}: 识别失败 ({response_time:.2f}s)")
                print(f"   💥 错误: {error or '未知错误'}")
                if timeout:
                    print(f"   ⏰ 超时")

            # 全局结果记录
            with threading.Lock():
                metrics.total_audio_files += 1
                metrics.response_times.append(response_time)
                if success:
                    metrics.successful_recognitions += 1
                    metrics.recognition_results.append({
                        "connection": connection_id,
                        "audio": os.path.basename(audio_file),
                        "text": result,
                        "response_time": response_time
                    })
                else:
                    error_entry = {
                        "connection": connection_id,
                        "audio": os.path.basename(audio_file),
                        "error": error or "未知错误",
                        "timeout": timeout
                    }
                    metrics.errors.append(error_entry)
            
            # 在下一个音频前随机等待
            if idx < len(audio_files) - 1:
                await asyncio.sleep(random.uniform(1.0, 3.0))
        
        # 关闭连接
        await websocket.close()
        
    except Exception as e:
        connection_results["errors"].append(f"连接错误: {str(e)}")
        error_entry = {
            "connection": connection_id,
            "error": str(e)
        }
        with threading.Lock():
            metrics.errors.append(error_entry)
    
    # 更新结束时间
    connection_results["end_time"] = time.time()
    connection_duration = connection_results["end_time"] - connection_results["start_time"]
    
    # 更新全局连接指标
    with threading.Lock():
        metrics.total_connections += 1
        metrics.connection_times.append(connection_duration)
        if connection_results["success"]:
            metrics.successful_connections += 1
        # 计算该连接的成功率
        successful_tests = sum(1 for t in connection_results["audio_tests"] if t["success"])
        connection_success_rate = successful_tests / len(connection_results["audio_tests"]) if connection_results["audio_tests"] else 0
        metrics.connection_success_rates.append(connection_success_rate)
    
    return connection_results

def print_test_progress(connections_done, total_connections):
    """打印测试进度"""
    progress = connections_done / total_connections * 100
    print(f"\r🚀 测试进度: {connections_done}/{total_connections} ({progress:.1f}%)", end="")
    if connections_done == total_connections:
        print()

async def wait_for_server_ready(server_url, max_wait_time=60):
    """等待服务器就绪"""
    health_url = server_url.replace("ws://", "http://").replace("ws/", "health")
    if not health_url.endswith("/health"):
        health_url = health_url.replace("/ws", "/health")
    
    print(f"🔍 检查服务器健康状态: {health_url}")
    
    start_time = time.time()
    while time.time() - start_time < max_wait_time:
        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(health_url, timeout=5) as response:
                    if response.status == 200:
                        data = await response.json()
                        if data.get("status") == "healthy":
                            print("✅ 服务器已就绪")
                            return True
                        else:
                            print(f"⏳ 服务器正在初始化: {data.get('status')}")
                    else:
                        print(f"⏳ 服务器响应状态码: {response.status}")
        except Exception as e:
            print(f"⏳ 等待服务器启动: {e}")
        
        await asyncio.sleep(2)
    
    print("❌ 服务器启动超时")
    return False

async def run_stress_test(config):
    """运行压力测试"""
    # 存储配置
    metrics.config = config
    metrics.start_time = time.time()
    
    # 等待服务器就绪
    if not await wait_for_server_ready(config["server_url"]):
        print("❌ 错误：服务器未就绪，测试终止")
        return
    
    # 获取音频文件
    audio_files = get_audio_files(config["audio_dir"])
    if not audio_files:
        print("❌ 错误：无有效音频文件，测试终止")
        return
    
    # 显示测试配置和音频文件
    print(f"\n🎯 压力测试配置:")
    print(f"  📡 服务器地址: {config['server_url']}")
    print(f"  🔗 并发连接数: {config['concurrent_connections']}")
    print(f"  🎵 每连接音频数: {config['audio_files_per_connection']}")
    print(f"  📁 音频文件目录: {config['audio_dir']}")
    print(f"  📋 可用音频文件:")
    for i, audio_file in enumerate(audio_files, 1):
        print(f"     {i}. {os.path.basename(audio_file)}")
    print(f"\n🚀 开始执行压力测试...")
    
    # 系统监控任务
    monitor_task = asyncio.create_task(system_monitor())
    
    # 为每个连接准备音频文件列表
    connection_tasks = []
    for i in range(config["concurrent_connections"]):
        selected_files = random.sample(audio_files, min(config["audio_files_per_connection"], len(audio_files)))
        connection_tasks.append(selected_files)
    
    # 创建并运行所有连接任务
    tasks = []
    for i, task_audio in enumerate(connection_tasks):
        task = asyncio.create_task(test_connection(i+1, task_audio, metrics))
        tasks.append(task)
    
    # 等待所有任务完成
    completed = 0
    for task in asyncio.as_completed(tasks):
        result = await task
        completed += 1
        print_test_progress(completed, len(tasks))
    
    # 完成系统监控
    monitor_task.cancel()
    try:
        await monitor_task
    except asyncio.CancelledError:
        pass
    
    # 记录结束时间
    metrics.end_time = time.time()
    # 保存结果（不再生成报告文件）
    # timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    # report_file = metrics.save_to_file(f"stress_test_report_{timestamp}.json")
    # print(f"\n📊 测试报告已保存至: {report_file}")

def print_summary():
    """打印测试摘要"""
    if not metrics.total_connections:
        print("❌ 错误：未执行测试")
        return
    print("\n🎯 VAD ASR 服务器压力测试结果摘要")

    duration = metrics.end_time - metrics.start_time
    print(f"⏱️  总测试时间: {duration:.2f}秒")
    print(f"🔌 并发连接数: {metrics.total_connections}")
    print(f"✅ 成功连接率: {metrics.successful_connections}/{metrics.total_connections} ({metrics.successful_connections/metrics.total_connections*100:.1f}%)")
    print(f"🎤 测试音频文件数: {metrics.total_audio_files}")
    print(f"🎯 成功识别率: {metrics.successful_recognitions}/{metrics.total_audio_files} ({metrics.successful_recognitions/metrics.total_audio_files*100:.1f}%)")

    # 响应时间统计
    if metrics.response_times:
        print(f"\n⏱️  响应时间统计:")
        print(f"  平均值: {statistics.mean(metrics.response_times):.2f}秒")
        print(f"  中位数: {statistics.median(metrics.response_times):.2f}秒")
        print(f"  最小值: {min(metrics.response_times):.2f}秒")
        print(f"  最大值: {max(metrics.response_times):.2f}秒")
        if len(metrics.response_times) > 4:
            print(f"  95百分位: {statistics.quantiles(metrics.response_times, n=100)[94]:.2f}秒")

    # 系统资源使用情况
    if metrics.system_stats:
        print(f"\n💻 系统资源使用情况:")
        cpu_avg = sum(stat["cpu"] for stat in metrics.system_stats) / len(metrics.system_stats)
        mem_avg = sum(stat["memory"] for stat in metrics.system_stats) / len(metrics.system_stats)
        print(f"  CPU平均使用率: {cpu_avg:.1f}%")
        print(f"  内存平均使用率: {mem_avg:.1f}%")
        net_total = sum(stat["network"] for stat in metrics.system_stats) / (1024 * 1024)
        print(f"  网络流量总量: {net_total:.2f} MB")

    # 详细识别结果统计
    print(f"\n📋 详细识别结果:")
    # 统计每个音频文件的识别数
    file_result_count = {}
    file_texts = {}
    for r in metrics.recognition_results:
        fname = r['audio']
        file_result_count[fname] = file_result_count.get(fname, 0) + 1
        file_texts.setdefault(fname, []).append(r['text'])
    for fname in sorted(file_result_count.keys()):
        print(f"   {fname}: {file_result_count[fname]}个识别结果")
        for text in file_texts[fname]:
            print(f"      └─ \"{text}\"")
    if not file_result_count:
        print("   ⚠️  没有识别到文本")

    # 性能评估
    print(f"\n🏆 性能评估:")
    success_rate = (metrics.successful_connections / metrics.total_connections) * 100 if metrics.total_connections else 0
    recognition_rate = (metrics.successful_recognitions / metrics.total_audio_files * 100) if metrics.total_audio_files else 0
    if metrics.successful_recognitions > 0:
        print("   🎉 VAD和ASR系统工作正常!")
        print("   ✅ 能够正确识别音频")
    else:
        print("   ⚠️  未检测到识别结果")
        print("   💡 可能需要调整VAD阈值或检查ASR模型")
    print(f"   📈 连接成功率: {success_rate:.1f}%")
    print(f"   🎤 识别率: {recognition_rate:.1f}%")

    # 错误统计
    if metrics.errors:
        print(f"\n❌ 错误报告 (总数: {len(metrics.errors)})")
        error_types = {}
        for error in metrics.errors:
            error_type = error.get("error", "未知错误")
            error_types[error_type] = error_types.get(error_type, 0) + 1
        print("  错误类型分布:")
        for error, count in error_types.items():
            print(f"    {error}: {count}次")

    print()

def main():
    # 设置信号处理
    signal.signal(signal.SIGINT, signal_handler)
    
    # 配置参数解析
    parser = argparse.ArgumentParser(description="VAD ASR 服务器压力测试工具")
    parser.add_argument("-c", "--connections", type=int, default=20, 
                        help="并发连接数 (默认: 20)")
    parser.add_argument("-a", "--audio-per-connection", type=int, default=3, 
                        dest="audio_per_connection",
                        help="每个连接的音频文件数 (默认: 3)")
    parser.add_argument("-d", "--audio-dir", default="test_wavs", 
                        help="音频文件目录 (默认: 'test_wavs')")
    parser.add_argument("-u", "--url", default="ws://localhost:8080/ws", 
                        help="服务器WebSocket URL (默认: 'ws://localhost:8080/ws')")
    parser.add_argument("-w", "--wait", type=float, default=3.0, 
                        help="发送完音频后的等待时间 (默认: 3.0秒)")
    
    args = parser.parse_args()
    
    # 配置设置
    config = {
        "concurrent_connections": args.connections,
        "audio_files_per_connection": args.audio_per_connection,
        "audio_dir": args.audio_dir,
        "server_url": args.url,
        "final_wait_time": args.wait
    }
    
    print("="*80)
    print("🔊 VAD ASR 服务器压力测试工具 (优化版)")
    print("="*80)
    print(f"服务器 URL: {config['server_url']}")
    print(f"并发连接数: {config['concurrent_connections']}")
    print(f"每连接音频数: {config['audio_files_per_connection']}")
    print(f"音频目录: {config['audio_dir']}")
    print("="*80)
    
    # 检查系统资源
    cpu_percent = psutil.cpu_percent(interval=1)
    mem = psutil.virtual_memory()
    if cpu_percent > 80:
        print(f"⚠️  警告: CPU使用率过高 ({cpu_percent}%)，测试可能不可靠")
    if mem.percent > 80:
        print(f"⚠️  警告: 内存使用率过高 ({mem.percent}%)，测试可能不可靠")
    
    # 运行测试并确保最终汇总一定输出
    try:
        asyncio.run(run_stress_test(config))
    except Exception as e:
        print(f"❌ 测试过程中发生异常: {e}")
    finally:
        print("\n" + "="*80)
        print("📝 压力测试最终汇总")
        print_summary()
        print("="*80)

if __name__ == "__main__":
    main()