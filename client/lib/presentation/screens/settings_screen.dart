import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:google_fonts/google_fonts.dart';

import '../../core/theme/premium_theme.dart';
import '../blocs/settings/settings_bloc.dart';

class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  final TextEditingController _connectController = TextEditingController();

  @override
  void dispose() {
    _connectController.dispose();
    super.dispose();
  }

  void _manualConnect() {
    final addr = _connectController.text.trim();
    if (addr.isNotEmpty) {
      context.read<SettingsBloc>().add(ConnectToPeerEvent(addr));
    }
  }

  void _copyToClipboard(String val, String message) {
    Clipboard.setData(ClipboardData(text: val));
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message, style: GoogleFonts.outfit(color: Colors.white)),
        backgroundColor: PremiumTheme.successGreen,
        duration: const Duration(seconds: 1),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('NODE CONFIGURATION', style: GoogleFonts.outfit(fontWeight: FontWeight.bold, letterSpacing: 1.2)),
        backgroundColor: Colors.transparent,
        elevation: 0,
      ),
      body: BlocConsumer<SettingsBloc, SettingsState>(
        listener: (context, state) {
          if (state is SettingsLoaded && state.connectStatus != null) {
            if (state.connectStatus == 'success') {
              _connectController.clear();
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('Connected successfully!', style: GoogleFonts.outfit()), backgroundColor: PremiumTheme.successGreen),
              );
              context.read<SettingsBloc>().add(LoadSettings()); // Refresh to update count
            } else if (state.connectStatus!.startsWith('failed')) {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('Connection failed: ${state.connectStatus}', style: GoogleFonts.outfit()), backgroundColor: PremiumTheme.errorRed),
              );
            }
          }
        },
        builder: (context, state) {
          if (state is SettingsLoading) {
            return const Center(child: CircularProgressIndicator(valueColor: AlwaysStoppedAnimation(PremiumTheme.neonCyan)));
          } else if (state is SettingsLoaded) {
            final shortPeerId = state.peerId.length > 20
                ? '${state.peerId.substring(0, 10)}...${state.peerId.substring(state.peerId.length - 10)}'
                : state.peerId;

            return SingleChildScrollView(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Identity Card
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              const Icon(Icons.perm_identity, color: PremiumTheme.neonCyan),
                              const SizedBox(width: 8),
                              Text('LOCAL PEER IDENTITY', style: GoogleFonts.outfit(fontWeight: FontWeight.bold, color: PremiumTheme.neonCyan)),
                            ],
                          ),
                          const Divider(height: 24, color: Colors.white10),
                          ListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text('My PeerID', style: GoogleFonts.outfit(color: PremiumTheme.textSecondary, fontSize: 13)),
                            subtitle: Text(shortPeerId, style: GoogleFonts.outfit(fontSize: 16, fontWeight: FontWeight.w600, color: PremiumTheme.textPrimary)),
                            trailing: IconButton(
                              icon: const Icon(Icons.copy, size: 18, color: PremiumTheme.textSecondary),
                              onPressed: () => _copyToClipboard(state.peerId, 'Copied Peer ID to clipboard'),
                            ),
                          ),
                          ListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text('Network Status', style: GoogleFonts.outfit(color: PremiumTheme.textSecondary, fontSize: 13)),
                            subtitle: Text(
                              '${state.peersCount} connected peers (${state.natType})',
                              style: GoogleFonts.outfit(fontSize: 15, color: PremiumTheme.successGreen, fontWeight: FontWeight.bold),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 16),

                  // Manual Dialer Card
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              const Icon(Icons.link, color: PremiumTheme.electricPurple),
                              const SizedBox(width: 8),
                              Text('DIAL REMOTE MULTIADDRESS', style: GoogleFonts.outfit(fontWeight: FontWeight.bold, color: PremiumTheme.electricPurple)),
                            ],
                          ),
                          const Divider(height: 24, color: Colors.white10),
                          TextField(
                            controller: _connectController,
                            style: GoogleFonts.outfit(fontSize: 14),
                            decoration: const InputDecoration(
                              hintText: '/ip4/127.0.0.1/tcp/10002/p2p/...',
                              labelText: 'Multiaddress',
                            ),
                          ),
                          const SizedBox(height: 16),
                          SizedBox(
                            width: double.infinity,
                            child: ElevatedButton(
                              onPressed: state.connectStatus == 'connecting' ? null : _manualConnect,
                              child: state.connectStatus == 'connecting'
                                  ? const SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2, valueColor: AlwaysStoppedAnimation(PremiumTheme.obsidianBg)))
                                  : const Text('CONNECT TO PEER'),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 16),

                  // Listen Addresses Section
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
                    child: Text(
                      'MY LISTEN MULTIADDRESSES',
                      style: GoogleFonts.outfit(color: PremiumTheme.neonCyan, fontSize: 12, fontWeight: FontWeight.bold, letterSpacing: 1.5),
                    ),
                  ),
                  if (state.listenAddresses.isEmpty)
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      child: Text('No addresses active', style: GoogleFonts.outfit(color: PremiumTheme.textSecondary)),
                    )
                  else
                    ...state.listenAddresses.map((addr) {
                      return Card(
                        margin: const EdgeInsets.symmetric(vertical: 4),
                        child: ListTile(
                          title: Text(addr, style: GoogleFonts.outfit(fontSize: 12, color: PremiumTheme.textPrimary)),
                          trailing: IconButton(
                            icon: const Icon(Icons.copy, size: 16, color: PremiumTheme.textSecondary),
                            onPressed: () => _copyToClipboard(addr, 'Copied Address to clipboard'),
                          ),
                        ),
                      );
                    }),
                ],
              ),
            );
          } else if (state is SettingsError) {
            return Center(child: Text(state.message, style: GoogleFonts.outfit(color: PremiumTheme.errorRed)));
          }
          return Container();
        },
      ),
    );
  }
}
