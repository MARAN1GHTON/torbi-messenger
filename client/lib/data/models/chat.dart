class Chat {
  final String id;
  final String type;
  final String peerId;
  final String lastMessage;

  Chat({
    required this.id,
    required this.type,
    required this.peerId,
    required this.lastMessage,
  });

  factory Chat.fromJson(Map<String, dynamic> json) {
    return Chat(
      id: json['id'] as String,
      type: json['type'] as String? ?? 'direct',
      peerId: json['peer_id'] as String? ?? '',
      lastMessage: json['last_message'] as String? ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'type': type,
      'peer_id': peerId,
      'last_message': lastMessage,
    };
  }

  Chat copyWith({String? lastMessage}) {
    return Chat(
      id: id,
      type: type,
      peerId: peerId,
      lastMessage: lastMessage ?? this.lastMessage,
    );
  }
}
