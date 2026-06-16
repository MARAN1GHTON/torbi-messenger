import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../data/repositories/auth_repository.dart';

// --- Events ---
abstract class AuthEvent {}

class CheckAuthSetup extends AuthEvent {}

class SetupPin extends AuthEvent {
  final String pin;
  SetupPin(this.pin);
}

class UnlockApp extends AuthEvent {
  final String pin;
  UnlockApp(this.pin);
}

class LockApp extends AuthEvent {}

// --- States ---
abstract class AuthState {}

class AuthInitial extends AuthState {}

class AuthSetupRequired extends AuthState {}

class AuthLocked extends AuthState {}

class AuthUnlocking extends AuthState {}

class AuthUnlocked extends AuthState {
  final int apiPort;
  AuthUnlocked(this.apiPort);
}

class AuthError extends AuthState {
  final String message;
  AuthError(this.message);
}

// --- Bloc ---
class AuthBloc extends Bloc<AuthEvent, AuthState> {
  final AuthRepository _authRepository;

  AuthBloc(this._authRepository) : super(AuthInitial()) {
    on<CheckAuthSetup>((event, emit) async {
      try {
        final setup = await _authRepository.isPinSetup();
        if (setup) {
          emit(AuthLocked());
        } else {
          emit(AuthSetupRequired());
        }
      } catch (e) {
        emit(AuthError('Failed to verify PIN status: $e'));
      }
    });

    on<SetupPin>((event, emit) async {
      emit(AuthUnlocking());
      try {
        // Unlock database and create it using the PIN
        final port = await _authRepository.unlockAndStart(event.pin);
        if (port > 0) {
          emit(AuthUnlocked(port));
        } else {
          emit(AuthError('Failed to initialize and encrypt the database.'));
        }
      } catch (e) {
        emit(AuthError('Failed during first-time setup: $e'));
      }
    });

    on<UnlockApp>((event, emit) async {
      emit(AuthUnlocking());
      try {
        final port = await _authRepository.unlockAndStart(event.pin);
        if (port > 0) {
          emit(AuthUnlocked(port));
        } else {
          emit(AuthError('Invalid PIN code. Please try again.'));
        }
      } catch (e) {
        emit(AuthError('Unlock error: $e'));
      }
    });

    on<LockApp>((event, emit) async {
      try {
        await _authRepository.lock();
        emit(AuthLocked());
      } catch (e) {
        emit(AuthError('Error locking application: $e'));
      }
    });
  }
}
