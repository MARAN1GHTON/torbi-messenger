import 'dart:async';

import '../../core/bridge/api_client.dart';
import '../models/chat.dart';
import '../models/message.dart';

class ChatRepository {
  final ApiClient _apiClient = ApiClient();

  /// Retrieve all active chat rooms.
  Future<List<Chat>> getChats() async {
    final rawChats = await _apiClient.getChats();
    return rawChats.map((c) => Chat.fromJson(c as Map<String, dynamic>)).toList();
  }

  /// Retrieve the message logs for a specific chat room.
  Future<List<Message>> getChatMessages(String chatId) async {
    final rawMsgs = await _apiClient.getChatMessages(chatId);
    return rawMsgs.map((m) => Message.fromJson(m as Map<String, dynamic>)).toList();
  }

  /// Start a new direct chat room session with a PeerID.
  Future<String> startChatWithPeer(String peerId) async {
    return await _apiClient.startChat(peerId);
  }

  /// Send a plaintext E2EE message into a chat room.
  Future<void> sendMessage(String chatId, String body) async {
    await _apiClient.sendChatMessage(chatId, body);
  }

  /// Emits incoming messages in real time.
  Stream<Message> get onMessageReceived {
    return _apiClient.eventStream
        .where((event) => event['event'] == 'message_received')
        .map((event) {
          final data = event['data'] as Map<String, dynamic>;
          final msgData = data['message'] as Map<String, dynamic>;
          return Message.fromJson(msgData);
        });
  }

  /// Emits peer sync completion status (useful to reload message logs).
  Stream<Map<String, String>> get onPeerSyncCompleted {
    return _apiClient.eventStream
        .where((event) => event['event'] == 'sync_done')
        .map((event) {
          final data = event['data'] as Map<String, dynamic>;
          return {
            'peer_id': data['peer_id'] as String? ?? '',
            'chat_id': data['chat_id'] as String? ?? '',
          };
        });
  }
}
