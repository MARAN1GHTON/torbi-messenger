import 'dart:async';

import '../../core/bridge/api_client.dart';
import '../models/peer.dart';

class NetworkRepository {
  final ApiClient _apiClient = ApiClient();

  /// Retrieve the current identity, listen addresses, connection counts, and NAT type.
  Future<Map<String, dynamic>> getNodeStatus() async {
    return await _apiClient.getStatus();
  }

  /// Retrieve all registered database peers with their live online/offline states.
  Future<List<Peer>> getPeers() async {
    final rawPeers = await _apiClient.getPeers();
    return rawPeers.map((p) => Peer.fromJson(p as Map<String, dynamic>)).toList();
  }

  /// Manually connect/dial a libp2p multiaddress.
  Future<void> connectToMultiaddress(String multiaddress) async {
    await _apiClient.connectToPeer(multiaddress);
  }

  /// Streams peer connectivity events (when a peer connects or disconnects).
  Stream<Peer> get onPeerStatusChanged {
    return _apiClient.eventStream
        .where((event) => event['event'] == 'peer_status')
        .map((event) {
          final data = event['data'] as Map<String, dynamic>;
          return Peer(
            id: data['peer_id'] as String,
            isOnline: data['is_online'] as bool? ?? false,
          );
        });
  }
}
