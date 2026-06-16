import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:google_fonts/google_fonts.dart';
import 'package:intl/intl.dart';

import '../../core/theme/premium_theme.dart';
import '../../data/models/message.dart';
import '../blocs/chat/chat_bloc.dart';

class ChatScreen extends StatefulWidget {
  final String chatId;
  final String peerId;

  const ChatScreen({super.key, required this.chatId, required this.peerId});

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  final TextEditingController _textController = TextEditingController();
  final ScrollController _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    // Load historical state and register callbacks
    context.read<ChatBloc>().add(EnterChat(chatId: widget.chatId, peerId: widget.peerId));
  }

  @override
  void dispose() {
    _textController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _sendMessage() {
    final body = _textController.text.trim();
    if (body.isNotEmpty) {
      context.read<ChatBloc>().add(SendMessageEvent(body));
      _textController.clear();
      // Animate scrolling down to show the newly sent message
      Timer(const Duration(milliseconds: 100), () {
        if (_scrollController.hasClients) {
          _scrollController.animateTo(
            _scrollController.position.maxScrollExtent,
            duration: const Duration(milliseconds: 200),
            curve: Curves.easeOut,
          );
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final shortPeerId = widget.peerId.length > 8 
        ? '...${widget.peerId.substring(widget.peerId.length - 8)}' 
        : widget.peerId;

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Chat with peer', style: GoogleFonts.outfit(fontWeight: FontWeight.w600, fontSize: 16)),
            Text(shortPeerId, style: GoogleFonts.outfit(fontSize: 12, color: PremiumTheme.textSecondary)),
          ],
        ),
        backgroundColor: PremiumTheme.cardBg,
        elevation: 1,
        actions: [
          // Synchronization Indicator
          BlocBuilder<ChatBloc, ChatState>(
            builder: (context, state) {
              if (state is ChatLoaded && state.isSyncing) {
                return Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  child: Row(
                    children: [
                      Text('Syncing', style: GoogleFonts.outfit(fontSize: 12, color: PremiumTheme.neonCyan)),
                      const SizedBox(width: 8),
                      const SizedBox(
                        width: 14,
                        height: 14,
                        child: CircularProgressIndicator(
                          strokeWidth: 1.5,
                          valueColor: AlwaysStoppedAnimation(PremiumTheme.neonCyan),
                        ),
                      ),
                    ],
                  ),
                );
              }
              return Container();
            },
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: BlocConsumer<ChatBloc, ChatState>(
              listener: (context, state) {
                // Scroll to bottom on load
                if (state is ChatLoaded) {
                  Timer(const Duration(milliseconds: 100), () {
                    if (_scrollController.hasClients) {
                      _scrollController.jumpTo(_scrollController.position.maxScrollExtent);
                    }
                  });
                }
              },
              builder: (context, state) {
                if (state is ChatLoading) {
                  return const Center(child: CircularProgressIndicator(valueColor: AlwaysStoppedAnimation(PremiumTheme.neonCyan)));
                } else if (state is ChatLoaded) {
                  final messages = state.messages;

                  if (messages.isEmpty) {
                    return Center(
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(Icons.lock_outline, size: 48, color: PremiumTheme.textSecondary.withAlpha(102)),
                          const SizedBox(height: 16),
                          Text('End-to-End Encrypted', style: GoogleFonts.outfit(color: PremiumTheme.textPrimary, fontWeight: FontWeight.bold)),
                          const SizedBox(height: 8),
                          Text('Messages are encrypted using ChaCha20-Poly1305', style: GoogleFonts.outfit(color: PremiumTheme.textSecondary, fontSize: 12)),
                        ],
                      ),
                    );
                  }

                  return ListView.builder(
                    controller: _scrollController,
                    padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 12),
                    itemCount: messages.length,
                    itemBuilder: (context, index) {
                      final msg = messages[index];
                      // Determine if sent by self (if msg.senderId doesn't match peerId)
                      final isSelf = msg.senderId != widget.peerId;
                      return _buildMessageBubble(msg, isSelf);
                    },
                  );
                } else if (state is ChatError) {
                  return Center(child: Text(state.message, style: GoogleFonts.outfit(color: PremiumTheme.errorRed)));
                }
                return Container();
              },
            ),
          ),
          _buildInputArea(),
        ],
      ),
    );
  }

  Widget _buildMessageBubble(Message msg, bool isSelf) {
    final formattedTime = DateFormat('HH:mm').format(msg.dateTime);
    return Align(
      alignment: isSelf ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.symmetric(vertical: 4),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.75),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(16),
            topRight: const Radius.circular(16),
            bottomLeft: Radius.circular(isSelf ? 16 : 4),
            bottomRight: Radius.circular(isSelf ? 4 : 16),
          ),
          gradient: isSelf
              ? const LinearGradient(
                  colors: [PremiumTheme.electricPurple, Color(0xFF6A0DAD)],
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                )
              : null,
          color: isSelf ? null : PremiumTheme.cardBg,
          border: isSelf ? null : Border.all(color: Colors.white.withAlpha(13), width: 1),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Text(
              msg.body,
              style: GoogleFonts.outfit(color: PremiumTheme.textPrimary, fontSize: 15),
            ),
            const SizedBox(height: 4),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  formattedTime,
                  style: GoogleFonts.outfit(color: PremiumTheme.textSecondary.withAlpha(179), fontSize: 10),
                ),
                const SizedBox(width: 4),
                // Lamport clock debug print
                Text(
                  'LC:${msg.lamportClock}',
                  style: GoogleFonts.outfit(color: PremiumTheme.neonCyan.withAlpha(153), fontSize: 9, fontWeight: FontWeight.bold),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInputArea() {
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 8, 12, 16),
      decoration: const BoxDecoration(
        color: PremiumTheme.cardBg,
        border: Border(top: BorderSide(color: Colors.white10, width: 1)),
      ),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: _textController,
              onSubmitted: (_) => _sendMessage(),
              style: GoogleFonts.outfit(color: PremiumTheme.textPrimary),
              decoration: InputDecoration(
                hintText: 'Type an encrypted message...',
                border: InputBorder.none,
                focusedBorder: InputBorder.none,
                enabledBorder: InputBorder.none,
                contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                fillColor: PremiumTheme.obsidianBg.withAlpha(102),
              ),
            ),
          ),
          const SizedBox(width: 8),
          IconButton(
            icon: const Icon(Icons.send, color: PremiumTheme.neonCyan),
            onPressed: _sendMessage,
          ),
        ],
      ),
    );
  }
}
