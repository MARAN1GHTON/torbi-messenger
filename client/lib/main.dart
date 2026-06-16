import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import 'core/theme/premium_theme.dart';
import 'data/repositories/auth_repository.dart';
import 'data/repositories/chat_repository.dart';
import 'data/repositories/network_repository.dart';
import 'presentation/blocs/auth/auth_bloc.dart';
import 'presentation/blocs/chat/chat_bloc.dart';
import 'presentation/blocs/chat_list/chat_list_bloc.dart';
import 'presentation/blocs/settings/settings_bloc.dart';
import 'presentation/screens/auth_screen.dart';
import 'presentation/screens/main_screen.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();

  // Set system navigation overlay styling
  SystemChrome.setSystemUIOverlayStyle(
    const SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: Brightness.light,
      systemNavigationBarColor: PremiumTheme.obsidianBg,
      systemNavigationBarIconBrightness: Brightness.light,
    ),
  );

  runApp(const TorbiApp());
}

class TorbiApp extends StatelessWidget {
  const TorbiApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MultiRepositoryProvider(
      providers: [
        RepositoryProvider<AuthRepository>(create: (_) => AuthRepository()),
        RepositoryProvider<ChatRepository>(create: (_) => ChatRepository()),
        RepositoryProvider<NetworkRepository>(create: (_) => NetworkRepository()),
      ],
      child: MultiBlocProvider(
        providers: [
          BlocProvider<AuthBloc>(
            create: (context) => AuthBloc(context.read<AuthRepository>())..add(CheckAuthSetup()),
          ),
          BlocProvider<ChatListBloc>(
            create: (context) => ChatListBloc(
              context.read<ChatRepository>(),
              context.read<NetworkRepository>(),
            ),
          ),
          BlocProvider<ChatBloc>(
            create: (context) => ChatBloc(context.read<ChatRepository>()),
          ),
          BlocProvider<SettingsBloc>(
            create: (context) => SettingsBloc(context.read<NetworkRepository>()),
          ),
        ],
        child: MaterialApp(
          title: 'Torbi Messenger',
          debugShowCheckedModeBanner: false,
          theme: PremiumTheme.darkTheme,
          home: const AppLifecycleShell(),
        ),
      ),
    );
  }
}

class AppLifecycleShell extends StatelessWidget {
  const AppLifecycleShell({super.key});

  @override
  Widget build(BuildContext context) {
    return BlocBuilder<AuthBloc, AuthState>(
      builder: (context, state) {
        if (state is AuthInitial) {
          return const Scaffold(
            body: Center(
              child: CircularProgressIndicator(
                valueColor: AlwaysStoppedAnimation(PremiumTheme.neonCyan),
              ),
            ),
          );
        } else if (state is AuthSetupRequired || state is AuthLocked || state is AuthUnlocking) {
          return const AuthScreen();
        } else if (state is AuthUnlocked) {
          return const MainScreen();
        } else {
          // Fallback view in case of load crash
          return Scaffold(
            body: Center(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  const Icon(Icons.error_outline, size: 48, color: PremiumTheme.errorRed),
                  const SizedBox(height: 16),
                  const Text(
                    'An initialization error occurred.',
                    style: TextStyle(color: PremiumTheme.textPrimary, fontSize: 16, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 12),
                  ElevatedButton(
                    onPressed: () {
                      context.read<AuthBloc>().add(CheckAuthSetup());
                    },
                    child: const Text('RETRY'),
                  ),
                ],
              ),
            ),
          );
        }
      },
    );
  }
}
