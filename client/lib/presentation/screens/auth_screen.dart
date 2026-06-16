import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:google_fonts/google_fonts.dart';

import '../../core/theme/premium_theme.dart';
import '../blocs/auth/auth_bloc.dart';

class AuthScreen extends StatefulWidget {
  const AuthScreen({super.key});

  @override
  State<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends State<AuthScreen> {
  String _pin = "";
  final int _pinLength = 4;

  void _onKeyPress(String val) {
    if (_pin.length < _pinLength) {
      setState(() {
        _pin += val;
      });
      if (_pin.length == _pinLength) {
        _submitPin();
      }
    }
  }

  void _onDelete() {
    if (_pin.isNotEmpty) {
      setState(() {
        _pin = _pin.substring(0, _pin.length - 1);
      });
    }
  }

  void _submitPin() {
    final authState = context.read<AuthBloc>().state;
    if (authState is AuthSetupRequired) {
      context.read<AuthBloc>().add(SetupPin(_pin));
    } else {
      context.read<AuthBloc>().add(UnlockApp(_pin));
    }
    setState(() {
      _pin = "";
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: BlocConsumer<AuthBloc, AuthState>(
        listener: (context, state) {
          if (state is AuthError) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text(state.message, style: GoogleFonts.outfit(color: Colors.white)),
                backgroundColor: PremiumTheme.errorRed,
                duration: const Duration(seconds: 2),
              ),
            );
          }
        },
        builder: (context, state) {
          final isSetup = state is AuthSetupRequired;
          final isLoading = state is AuthUnlocking;

          return Container(
            width: double.infinity,
            height: double.infinity,
            decoration: BoxDecoration(
              gradient: RadialGradient(
                center: const Alignment(0, -0.3),
                radius: 1.2,
                colors: [
                  PremiumTheme.electricPurple.withAlpha(38),
                  PremiumTheme.obsidianBg,
                ],
              ),
            ),
            child: SafeArea(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  const Spacer(),
                  // Branding Logo / Icon
                  Container(
                    width: 72,
                    height: 72,
                    decoration: BoxDecoration(
                      color: PremiumTheme.neonCyan.withAlpha(26),
                      shape: BoxShape.circle,
                      border: Border.all(color: PremiumTheme.neonCyan.withAlpha(128), width: 1.5),
                    ),
                    child: const Icon(Icons.security, size: 36, color: PremiumTheme.neonCyan),
                  ),
                  const SizedBox(height: 24),
                  Text(
                    'TORBI P2P',
                    style: GoogleFonts.outfit(
                      fontSize: 28,
                      fontWeight: FontWeight.bold,
                      letterSpacing: 2.0,
                      color: PremiumTheme.textPrimary,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    isSetup ? 'Setup database decryption PIN' : 'Enter PIN to decrypt database',
                    style: GoogleFonts.outfit(color: PremiumTheme.textSecondary, fontSize: 14),
                  ),
                  const SizedBox(height: 32),
                  // PIN Indicators (Dots)
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: List.generate(_pinLength, (index) {
                      final active = index < _pin.length;
                      return AnimatedContainer(
                        duration: const Duration(milliseconds: 150),
                        margin: const EdgeInsets.symmetric(horizontal: 10),
                        width: 16,
                        height: 16,
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: active ? PremiumTheme.neonCyan : Colors.transparent,
                          border: Border.all(
                            color: active ? PremiumTheme.neonCyan : PremiumTheme.textSecondary.withAlpha(128),
                            width: 1.5,
                          ),
                          boxShadow: active
                              ? [BoxShadow(color: PremiumTheme.neonCyan.withAlpha(153), blurRadius: 8, spreadRadius: 1)]
                              : [],
                        ),
                      );
                    }),
                  ),
                  const Spacer(),
                  if (isLoading)
                    const Padding(
                      padding: EdgeInsets.all(32.0),
                      child: CircularProgressIndicator(valueColor: AlwaysStoppedAnimation(PremiumTheme.neonCyan)),
                    )
                  else
                    // Numeric Keyboard Pad
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 32),
                      constraints: const BoxConstraints(maxWidth: 400),
                      child: Column(
                        children: [
                          for (var row in [
                            ['1', '2', '3'],
                            ['4', '5', '6'],
                            ['7', '8', '9'],
                          ])
                            Padding(
                              padding: const EdgeInsets.symmetric(vertical: 8),
                              child: Row(
                                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                                children: row.map((val) => _buildKey(val)).toList(),
                              ),
                            ),
                          Padding(
                            padding: const EdgeInsets.symmetric(vertical: 8),
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                              children: [
                                const SizedBox(width: 72, height: 72), // Empty placeholder
                                _buildKey('0'),
                                _buildDeleteKey(),
                              ],
                            ),
                          )
                        ],
                      ),
                    ),
                  const SizedBox(height: 48),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildKey(String val) {
    return InkWell(
      onTap: () => _onKeyPress(val),
      borderRadius: BorderRadius.circular(36),
      child: Container(
        width: 72,
        height: 72,
        decoration: BoxDecoration(
          color: Colors.white.withAlpha(5),
          shape: BoxShape.circle,
          border: Border.all(color: Colors.white.withAlpha(10), width: 1.5),
        ),
        alignment: Alignment.center,
        child: Text(
          val,
          style: GoogleFonts.outfit(fontSize: 28, fontWeight: FontWeight.w500, color: PremiumTheme.textPrimary),
        ),
      ),
    );
  }

  Widget _buildDeleteKey() {
    return InkWell(
      onTap: _onDelete,
      borderRadius: BorderRadius.circular(36),
      child: Container(
        width: 72,
        height: 72,
        alignment: Alignment.center,
        child: const Icon(Icons.backspace_outlined, size: 28, color: PremiumTheme.textSecondary),
      ),
    );
  }
}
