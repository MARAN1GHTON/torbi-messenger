import 'dart:async';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../data/models/message.dart';
import '../../../data/repositories/chat_repository.dart';

// --- Events ---
abstract class ChatEvent {}

class EnterChat extends ChatEvent {
  final String chatId;
  final String peerId;
  EnterChat({required this.chatId, required this.peerId});
}

class SendMessageEvent extends ChatEvent {
  final String body;
  SendMessageEvent(this.body);
}

class MessageReceivedEvent extends ChatEvent {
  final Message message;
  MessageReceivedEvent(this.message);
}

class ReloadChatOnSync extends ChatEvent {}

// --- States ---
abstract class ChatState {}

class ChatInitialState extends ChatState {}

class ChatLoading extends ChatState {}

class ChatLoaded extends ChatState {
  final String chatId;
  final String peerId;
  final List<Message> messages;
  final bool isSyncing;

  ChatLoaded({
    required this.chatId,
    required this.peerId,
    required this.messages,
    this.isSyncing = false,
  });

  ChatLoaded copyWith({
    List<Message>? messages,
    bool? isSyncing,
  }) {
    return ChatLoaded(
      chatId: chatId,
      peerId: peerId,
      messages: messages ?? this.messages,
      isSyncing: isSyncing ?? this.isSyncing,
    );
  }
}

class ChatError extends ChatState {
  final String message;
  ChatError(this.message);
}

// --- Bloc ---
class ChatBloc extends Bloc<ChatEvent, ChatState> {
  final ChatRepository _chatRepository;
  
  StreamSubscription? _msgSubscription;
  StreamSubscription? _syncSubscription;
  String? _activeChatId;

  ChatBloc(this._chatRepository) : super(ChatInitialState()) {
    on<EnterChat>((event, emit) async {
      emit(ChatLoading());
      _activeChatId = event.chatId;
      
      // Cancel previous subscriptions if any
      _msgSubscription?.cancel();
      _syncSubscription?.cancel();

      try {
        final messages = await _chatRepository.getChatMessages(event.chatId);
        emit(ChatLoaded(
          chatId: event.chatId,
          peerId: event.peerId,
          messages: messages,
          isSyncing: true, // Mark syncing initially upon peer loading
        ));

        // 1. Subscribe to incoming messages for this chat
        _msgSubscription = _chatRepository.onMessageReceived
            .where((msg) => msg.chatId == event.chatId)
            .listen((msg) {
              add(MessageReceivedEvent(msg));
            });

        // 2. Subscribe to sync completion to reload history and hide loading indicators
        _syncSubscription = _chatRepository.onPeerSyncCompleted
            .where((syncData) => syncData['chat_id'] == event.chatId)
            .listen((_) {
              add(ReloadChatOnSync());
            });

        // Auto-clear syncing overlay after a safety timeout of 3 seconds if no event fires
        Future.delayed(const Duration(seconds: 3), () {
          if (_activeChatId == event.chatId && !isClosed) {
            add(ReloadChatOnSync());
          }
        });

      } catch (e) {
        emit(ChatError('Failed to load chat history: $e'));
      }
    });

    on<SendMessageEvent>((event, emit) async {
      if (state is ChatLoaded) {
        final current = state as ChatLoaded;
        try {
          // Send to the backend
          await _chatRepository.sendMessage(current.chatId, event.body);
          
          // Refresh list from database to verify ingestion
          final messages = await _chatRepository.getChatMessages(current.chatId);
          emit(current.copyWith(messages: messages));
        } catch (e) {
          emit(ChatError('Failed to send message: $e'));
        }
      }
    });

    on<MessageReceivedEvent>((event, emit) {
      if (state is ChatLoaded) {
        final current = state as ChatLoaded;
        
        // Prevent duplicate appends
        if (current.messages.any((m) => m.id == event.message.id)) {
          return;
        }

        final updatedMessages = List<Message>.from(current.messages)..add(event.message);
        
        // Order properly by Lamport Clock
        updatedMessages.sort((a, b) {
          final cmp = a.lamportClock.compareTo(b.lamportClock);
          if (cmp != 0) return cmp;
          return a.timestamp.compareTo(b.timestamp);
        });

        emit(current.copyWith(messages: updatedMessages));
      }
    });

    on<ReloadChatOnSync>((event, emit) async {
      if (state is ChatLoaded && _activeChatId != null) {
        final current = state as ChatLoaded;
        try {
          final messages = await _chatRepository.getChatMessages(_activeChatId!);
          emit(ChatLoaded(
            chatId: current.chatId,
            peerId: current.peerId,
            messages: messages,
            isSyncing: false, // Turn off syncing indicator
          ));
        } catch (_) {}
      }
    });
  }

  @override
  Future<void> close() {
    _msgSubscription?.cancel();
    _syncSubscription?.cancel();
    return super.close();
  }
}
