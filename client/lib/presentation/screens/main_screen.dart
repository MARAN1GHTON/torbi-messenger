import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:google_fonts/google_fonts.dart';

import '../../core/theme/premium_theme.dart';
import '../../data/models/chat.dart';
import '../../data/models/peer.dart';
import '../../data/repositories/chat_repository.dart';
import '../blocs/chat_list/chat_list_bloc.dart';
import '../blocs/settings/settings_bloc.dart';
import 'chat_screen.dart';
import 'settings_screen.dart';

class MainScreen extends StatefulWidget {
  const MainScreen({super.key});

  @override
  State<MainScreen> createState() => _MainScreenState();
}

class _MainScreenState extends State<MainScreen> {
  int _currentIndex = 0;

  @override
  void initState() {
    super.initState();
    // Load data upon screen entrance
    context.read<ChatListBloc>().add(LoadChatsAndPeers());
    context.read<SettingsBloc>().add(LoadSettings());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: LayoutBuilder(
        builder: (context, constraints) {
          final isDesktop = constraints.maxWidth >= 600;

          if (isDesktop) {
            return Row(
              children: [
                // Desktop Sidebar Rail
                NavigationRail(
                  selectedIndex: _currentIndex,
                  onDestinationSelected: (idx) {
                    setState(() {
                      _currentIndex = idx;
                    });
                  },
                  backgroundColor: PremiumTheme.cardBg,
                  indicatorColor: PremiumTheme.neonCyan.withAlpha(38),
                  selectedIconTheme: const IconThemeData(color: PremiumTheme.neonCyan),
                  unselectedIconTheme: const IconThemeData(color: PremiumTheme.textSecondary),
                  selectedLabelTextStyle: GoogleFonts.outfit(color: PremiumTheme.neonCyan, fontWeight: FontWeight.bold),
                  unselectedLabelTextStyle: GoogleFonts.outfit(color: PremiumTheme.textSecondary),
                  labelType: NavigationRailLabelType.all,
                  destinations: const [
                    NavigationRailDestination(
                      icon: Icon(Icons.chat_bubble_outline),
                      selectedIcon: Icon(Icons.chat_bubble),
                      label: Text('Chats'),
                    ),
                    NavigationRailDestination(
                      icon: Icon(Icons.settings_outlined),
                      selectedIcon: Icon(Icons.settings),
                      label: Text('Settings'),
                    ),
                  ],
                ),
                const VerticalDivider(width: 1, color: Colors.white10),
                Expanded(
                  child: _buildBody(_currentIndex),
                ),
              ],
            );
          } else {
            // Mobile Bottom Navigation
            return Scaffold(
              body: _buildBody(_currentIndex),
              bottomNavigationBar: BottomNavigationBar(
                currentIndex: _currentIndex,
                onTap: (idx) {
                  setState(() {
                    _currentIndex = idx;
                  });
                },
                backgroundColor: PremiumTheme.cardBg,
                selectedItemColor: PremiumTheme.neonCyan,
                unselectedItemColor: PremiumTheme.textSecondary,
                selectedLabelStyle: GoogleFonts.outfit(fontWeight: FontWeight.bold),
                unselectedLabelStyle: GoogleFonts.outfit(),
                items: const [
                  BottomNavigationBarItem(
                    icon: Icon(Icons.chat_bubble_outline),
                    activeIcon: Icon(Icons.chat_bubble),
                    label: 'Chats',
                  ),
                  BottomNavigationBarItem(
                    icon: Icon(Icons.settings_outlined),
                    activeIcon: Icon(Icons.settings),
                    label: 'Settings',
                  ),
                ],
              ),
            );
          }
        },
      ),
    );
  }

  Widget _buildBody(int index) {
    switch (index) {
      case 0:
        return const _ChatsTab();
      case 1:
        return const SettingsScreen();
      default:
        return const _ChatsTab();
    }
  }
}

class _ChatsTab extends StatelessWidget {
  const _ChatsTab();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('TORBI MESSENGER', style: GoogleFonts.outfit(fontWeight: FontWeight.bold, letterSpacing: 1.2)),
        backgroundColor: Colors.transparent,
        elevation: 0,
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh, color: PremiumTheme.neonCyan),
            onPressed: () {
              context.read<ChatListBloc>().add(LoadChatsAndPeers());
            },
          ),
        ],
      ),
      body: BlocBuilder<ChatListBloc, ChatListState>(
        builder: (context, state) {
          if (state is ChatListLoading) {
            return const Center(child: CircularProgressIndicator(valueColor: AlwaysStoppedAnimation(PremiumTheme.neonCyan)));
          } else if (state is ChatListLoaded) {
            final chats = state.chats;
            final peers = state.peers;

            if (chats.isEmpty && peers.isEmpty) {
              return Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(Icons.wifi_off_outlined, size: 64, color: PremiumTheme.textSecondary.withAlpha(128)),
                    const SizedBox(height: 16),
                    Text('No local peers found yet', style: GoogleFonts.outfit(color: PremiumTheme.textPrimary, fontSize: 18, fontWeight: FontWeight.bold)),
                    const SizedBox(height: 8),
                    Text('Waiting for mDNS discovery on local network...', style: GoogleFonts.outfit(color: PremiumTheme.textSecondary)),
                  ],
                ),
              );
            }

            return CustomScrollView(
              slivers: [
                // Online/Discovered Peers Section
                if (peers.isNotEmpty) ...[
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
                      child: Text(
                        'DISCOVERED PEERS (${peers.length})',
                        style: GoogleFonts.outfit(color: PremiumTheme.neonCyan, fontSize: 12, fontWeight: FontWeight.bold, letterSpacing: 1.5),
                      ),
                    ),
                  ),
                  SliverToBoxAdapter(
                    child: SizedBox(
                      height: 100,
                      child: ListView.builder(
                        scrollDirection: Axis.horizontal,
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                        itemCount: peers.length,
                        itemBuilder: (context, index) {
                          final peer = peers[index];
                          return _buildPeerAvatar(context, peer);
                        },
                      ),
                    ),
                  ),
                ],

                // Active Chats Section
                SliverToBoxAdapter(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(16, 24, 16, 8),
                    child: Text(
                      'CONVERSATIONS',
                      style: GoogleFonts.outfit(color: PremiumTheme.neonCyan, fontSize: 12, fontWeight: FontWeight.bold, letterSpacing: 1.5),
                    ),
                  ),
                ),
                if (chats.isEmpty)
                  SliverToBoxAdapter(
                    child: Padding(
                      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 16),
                      child: Center(
                        child: Text(
                          'Tap a discovered peer above to start a chat.',
                          style: GoogleFonts.outfit(color: PremiumTheme.textSecondary, fontSize: 14),
                        ),
                      ),
                    ),
                  )
                else
                  SliverList(
                    delegate: SliverChildBuilderImpl(
                      (context, index) {
                        final chat = chats[index];
                        return _buildChatTile(context, chat);
                      },
                      childCount: chats.length,
                    ),
                  ),
              ],
            );
          } else if (state is ChatListError) {
            return Center(child: Text(state.message, style: GoogleFonts.outfit(color: PremiumTheme.errorRed)));
          }
          return Container();
        },
      ),
    );
  }

  Widget _buildPeerAvatar(BuildContext context, Peer peer) {
    final nameSnippet = peer.id.length > 8 ? peer.id.substring(peer.id.length - 8) : peer.id;
    return GestureDetector(
      onTap: () async {
        try {
          // Trigger chat generation in background and enter screen
          final chatId = await context.read<ChatRepository>().startChatWithPeer(peer.id);
          if (context.mounted) {
            Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => ChatScreen(chatId: chatId, peerId: peer.id),
              ),
            );
          }
        } catch (e) {
          if (context.mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text('Failed to initiate chat: $e'), backgroundColor: PremiumTheme.errorRed),
            );
          }
        }
      },
      child: Container(
        width: 80,
        margin: const EdgeInsets.symmetric(horizontal: 6, vertical: 8),
        padding: const EdgeInsets.symmetric(vertical: 8),
        decoration: PremiumTheme.glassDecoration(),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Stack(
              children: [
                CircleAvatar(
                  radius: 20,
                  backgroundColor: PremiumTheme.neonCyan.withAlpha(26),
                  child: const Icon(Icons.person, color: PremiumTheme.neonCyan, size: 20),
                ),
                Positioned(
                  right: 0,
                  bottom: 0,
                  child: Container(
                    width: 12,
                    height: 12,
                    decoration: BoxDecoration(
                      color: peer.isOnline ? PremiumTheme.successGreen : PremiumTheme.textSecondary,
                      shape: BoxShape.circle,
                      border: Border.all(color: PremiumTheme.obsidianBg, width: 2),
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Text(
              nameSnippet,
              style: GoogleFonts.outfit(fontSize: 11, color: PremiumTheme.textPrimary, fontWeight: FontWeight.w500),
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildChatTile(BuildContext context, Chat chat) {
    final shortPeerId = chat.peerId.length > 12 
        ? '...${chat.peerId.substring(chat.peerId.length - 12)}' 
        : chat.peerId;
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      child: ListTile(
        onTap: () {
          Navigator.of(context).push(
            MaterialPageRoute(
              builder: (_) => ChatScreen(chatId: chat.id, peerId: chat.peerId),
            ),
          );
        },
        leading: CircleAvatar(
          backgroundColor: PremiumTheme.electricPurple.withAlpha(26),
          child: const Icon(Icons.forum_outlined, color: PremiumTheme.electricPurple),
        ),
        title: Text(
          'Peer ID: $shortPeerId',
          style: GoogleFonts.outfit(fontWeight: FontWeight.w600, color: PremiumTheme.textPrimary),
        ),
        subtitle: Text(
          chat.lastMessage.isEmpty ? 'No messages yet' : chat.lastMessage,
          style: GoogleFonts.outfit(color: PremiumTheme.textSecondary),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        trailing: const Icon(Icons.chevron_right, color: PremiumTheme.textSecondary, size: 20),
      ),
    );
  }
}

// Simple SliverList helper
class SliverChildBuilderImpl extends SliverChildDelegate {
  final Widget? Function(BuildContext, int) builder;
  final int childCount;

  SliverChildBuilderImpl(this.builder, {required this.childCount});

  @override
  Widget? build(BuildContext context, int index) {
    if (index < 0 || index >= childCount) return null;
    return builder(context, index);
  }

  @override
  bool shouldRebuild(covariant SliverChildBuilderImpl oldDelegate) {
    return oldDelegate.childCount != childCount;
  }

  @override
  int? get estimatedChildCount => childCount;
}
