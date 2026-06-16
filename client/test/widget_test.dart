import 'package:flutter_test/flutter_test.dart';
import 'package:client/data/models/peer.dart';
import 'package:client/data/models/chat.dart';
import 'package:client/data/models/message.dart';

void main() {
  group('Torbi Data Models Tests', () {
    test('Peer model serialization', () {
      final json = {'id': 'peer_123', 'is_online': true};
      final peer = Peer.fromJson(json);

      expect(peer.id, 'peer_123');
      expect(peer.isOnline, true);

      final serialized = peer.toJson();
      expect(serialized['id'], 'peer_123');
      expect(serialized['is_online'], true);
    });

    test('Chat model serialization', () {
      final json = {
        'id': 'local_remote',
        'type': 'direct',
        'peer_id': 'remote_123',
        'last_message': 'hello'
      };
      final chat = Chat.fromJson(json);

      expect(chat.id, 'local_remote');
      expect(chat.peerId, 'remote_123');
      expect(chat.lastMessage, 'hello');

      final serialized = chat.toJson();
      expect(serialized['id'], 'local_remote');
      expect(serialized['peer_id'], 'remote_123');
    });

    test('Message model serialization', () {
      final json = {
        'id': 'msg_001',
        'chat_id': 'chat_001',
        'sender_id': 'peer_123',
        'body': 'E2EE payload',
        'timestamp': 1718559000000,
        'lamport_clock': 10
      };
      final msg = Message.fromJson(json);

      expect(msg.id, 'msg_001');
      expect(msg.body, 'E2EE payload');
      expect(msg.lamportClock, 10);
      expect(msg.dateTime.millisecondsSinceEpoch, 1718559000000);

      final serialized = msg.toJson();
      expect(serialized['id'], 'msg_001');
      expect(serialized['lamport_clock'], 10);
    });
  });
}
