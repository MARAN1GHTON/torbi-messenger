class Message {
  final String id;
  final String chatId;
  final String senderId;
  final String body;
  final int timestamp;
  final int lamportClock;

  Message({
    required this.id,
    required this.chatId,
    required this.senderId,
    required this.body,
    required this.timestamp,
    required this.lamportClock,
  });

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] as String,
      chatId: json['chat_id'] as String,
      senderId: json['sender_id'] as String,
      body: json['body'] as String? ?? '',
      timestamp: json['timestamp'] as int? ?? 0,
      lamportClock: json['lamport_clock'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'chat_id': chatId,
      'sender_id': senderId,
      'body': body,
      'timestamp': timestamp,
      'lamport_clock': lamportClock,
    };
  }

  DateTime get dateTime => DateTime.fromMillisecondsSinceEpoch(timestamp);
}
