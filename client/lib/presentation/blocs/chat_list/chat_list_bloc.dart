import 'dart:async';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../data/models/chat.dart';
import '../../../data/models/peer.dart';
import '../../../data/repositories/chat_repository.dart';
import '../../../data/repositories/network_repository.dart';

// --- Events ---
abstract class ChatListEvent {}

class LoadChatsAndPeers extends ChatListEvent {}

class UpdatePeerStatus extends ChatListEvent {
  final Peer peer;
  UpdatePeerStatus(this.peer);
}

class RefreshOnSync extends ChatListEvent {
  final String chatId;
  RefreshOnSync(this.chatId);
}

class UpdateLastMessagePreview extends ChatListEvent {
  final String chatId;
  final String snippet;
  UpdateLastMessagePreview({required this.chatId, required this.snippet});
}

// --- States ---
abstract class ChatListState {}

class ChatListLoading extends ChatListState {}

class ChatListLoaded extends ChatListState {
  final List<Chat> chats;
  final List<Peer> peers;
  ChatListLoaded({required this.chats, required this.peers});
}

class ChatListError extends ChatListState {
  final String message;
  ChatListError(this.message);
}

// --- Bloc ---
class ChatListBloc extends Bloc<ChatListEvent, ChatListState> {
  final ChatRepository _chatRepository;
  final NetworkRepository _networkRepository;
  
  StreamSubscription? _peerStatusSubscription;
  StreamSubscription? _syncSubscription;
  StreamSubscription? _msgSubscription;

  ChatListBloc(this._chatRepository, this._networkRepository) : super(ChatListLoading()) {
    on<LoadChatsAndPeers>((event, emit) async {
      try {
        final chats = await _chatRepository.getChats();
        final peers = await _networkRepository.getPeers();
        emit(ChatListLoaded(chats: chats, peers: peers));
      } catch (e) {
        emit(ChatListError('Failed to load dashboard data: $e'));
      }
    });

    on<UpdatePeerStatus>((event, emit) {
      if (state is ChatListLoaded) {
        final current = state as ChatListLoaded;
        final updatedPeers = List<Peer>.from(current.peers);
        final index = updatedPeers.indexWhere((p) => p.id == event.peer.id);
        
        if (index != -1) {
          updatedPeers[index] = event.peer;
        } else {
          updatedPeers.add(event.peer);
        }

        emit(ChatListLoaded(chats: current.chats, peers: updatedPeers));
      }
    });

    on<RefreshOnSync>((event, emit) async {
      if (state is ChatListLoaded) {
        try {
          final chats = await _chatRepository.getChats();
          final current = state as ChatListLoaded;
          emit(ChatListLoaded(chats: chats, peers: current.peers));
        } catch (_) {}
      }
    });

    on<UpdateLastMessagePreview>((event, emit) {
      if (state is ChatListLoaded) {
        final current = state as ChatListLoaded;
        final updatedChats = current.chats.map((c) {
          if (c.id == event.chatId) {
            return c.copyWith(lastMessage: event.snippet);
          }
          return c;
        }).toList();

        // Sort chats to move updated one to top (standard messenger UX)
        updatedChats.sort((a, b) {
          if (a.id == event.chatId) return -1;
          if (b.id == event.chatId) return 1;
          return 0;
        });

        emit(ChatListLoaded(chats: updatedChats, peers: current.peers));
      }
    });

    // 1. Subscribe to network status changes
    _peerStatusSubscription = _networkRepository.onPeerStatusChanged.listen((peer) {
      add(UpdatePeerStatus(peer));
    });

    // 2. Subscribe to synchronization completion signals
    _syncSubscription = _chatRepository.onPeerSyncCompleted.listen((syncData) {
      add(RefreshOnSync(syncData['chat_id'] ?? ''));
    });

    // 3. Subscribe to real-time message streams
    _msgSubscription = _chatRepository.onMessageReceived.listen((msg) {
      add(UpdateLastMessagePreview(chatId: msg.chatId, snippet: msg.body));
      add(LoadChatsAndPeers()); // Auto-load to pull new chats if they are generated
    });
  }

  @override
  Future<void> close() {
    _peerStatusSubscription?.cancel();
    _syncSubscription?.cancel();
    _msgSubscription?.cancel();
    return super.close();
  }
}
