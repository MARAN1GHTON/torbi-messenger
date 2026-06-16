class Peer {
  final String id;
  final bool isOnline;

  Peer({required this.id, required this.isOnline});

  factory Peer.fromJson(Map<String, dynamic> json) {
    return Peer(
      id: json['id'] as String,
      isOnline: json['is_online'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'is_online': isOnline,
    };
  }

  Peer copyWith({bool? isOnline}) {
    return Peer(
      id: id,
      isOnline: isOnline ?? this.isOnline,
    );
  }
}
