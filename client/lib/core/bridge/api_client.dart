import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:web_socket_channel/web_socket_channel.dart';

class ApiClient {
  static final ApiClient _instance = ApiClient._internal();
  factory ApiClient() => _instance;
  ApiClient._internal();

  int? _port;
  WebSocketChannel? _wsChannel;
  final StreamController<Map<String, dynamic>> _eventStreamController = StreamController<Map<String, dynamic>>.broadcast();

  void initialize(int port) {
    _port = port;
    _connectWebSocket();
  }

  int get port => _port ?? (throw StateError('ApiClient not initialized. Call initialize() first.'));

  String get _baseHttpUrl => 'http://127.0.0.1:$port';
  String get _baseWsUrl => 'ws://127.0.0.1:$port/ws';

  void _connectWebSocket() {
    _wsChannel?.sink.close();
    try {
      _wsChannel = WebSocketChannel.connect(Uri.parse(_baseWsUrl));
      _wsChannel!.stream.listen(
        (message) {
          try {
            final decoded = jsonDecode(message as String) as Map<String, dynamic>;
            _eventStreamController.add(decoded);
          } catch (e) {
            // Ignore malformed JSON
          }
        },
        onError: (err) {
          // Retry connection after delay
          Future.delayed(const Duration(seconds: 2), _connectWebSocket);
        },
        onDone: () {
          // Retry connection after delay
          Future.delayed(const Duration(seconds: 2), _connectWebSocket);
        },
      );
    } catch (e) {
      Future.delayed(const Duration(seconds: 2), _connectWebSocket);
    }
  }

  Stream<Map<String, dynamic>> get eventStream => _eventStreamController.stream;

  Future<Map<String, dynamic>> getStatus() async {
    final response = await http.get(Uri.parse('$_baseHttpUrl/status'));
    if (response.statusCode == 200) {
      return jsonDecode(response.body) as Map<String, dynamic>;
    } else {
      throw Exception('Failed to fetch status: ${response.statusCode}');
    }
  }

  Future<List<dynamic>> getPeers() async {
    final response = await http.get(Uri.parse('$_baseHttpUrl/peers'));
    if (response.statusCode == 200) {
      return jsonDecode(response.body) as List<dynamic>;
    } else {
      throw Exception('Failed to fetch peers: ${response.statusCode}');
    }
  }

  Future<List<dynamic>> getChats() async {
    final response = await http.get(Uri.parse('$_baseHttpUrl/chats'));
    if (response.statusCode == 200) {
      return jsonDecode(response.body) as List<dynamic>;
    } else {
      throw Exception('Failed to fetch chats: ${response.statusCode}');
    }
  }

  Future<List<dynamic>> getChatMessages(String chatId) async {
    final response = await http.get(Uri.parse('$_baseHttpUrl/chats/$chatId/messages'));
    if (response.statusCode == 200) {
      return jsonDecode(response.body) as List<dynamic>;
    } else {
      throw Exception('Failed to fetch chat messages: ${response.statusCode}');
    }
  }

  Future<void> sendChatMessage(String chatId, String body) async {
    final response = await http.post(
      Uri.parse('$_baseHttpUrl/chats/$chatId/messages'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'body': body}),
    );
    if (response.statusCode != 200) {
      throw Exception('Failed to send message: ${response.statusCode} - ${response.body}');
    }
  }

  Future<void> connectToPeer(String multiaddr) async {
    final response = await http.post(
      Uri.parse('$_baseHttpUrl/connect'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'multiaddr': multiaddr}),
    );
    if (response.statusCode != 200) {
      throw Exception('Failed to connect to peer: ${response.statusCode} - ${response.body}');
    }
  }

  Future<String> startChat(String peerId) async {
    final response = await http.post(
      Uri.parse('$_baseHttpUrl/chat'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({'peer_id': peerId}),
    );
    if (response.statusCode == 200) {
      final decoded = jsonDecode(response.body) as Map<String, dynamic>;
      return decoded['chat_id'] as String;
    } else {
      throw Exception('Failed to start chat: ${response.statusCode} - ${response.body}');
    }
  }

  void close() {
    _wsChannel?.sink.close();
  }
}
