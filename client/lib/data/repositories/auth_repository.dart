import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'package:crypto/crypto.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

import '../../core/bridge/api_client.dart';
import '../../core/bridge/torbi_bridge.dart';

class AuthRepository {
  final _secureStorage = const FlutterSecureStorage();
  static const _pinSetupKey = 'torbi_pin_setup';
  static const _dbKeyName = 'torbi_db_pass';
  static const _pinHashName = 'torbi_pin_hash';

  /// Check if the user has already configured a PIN.
  Future<bool> isPinSetup() async {
    final setup = await _secureStorage.read(key: _pinSetupKey);
    return setup == 'true';
  }

  /// Registers a new PIN in the system secure storage.
  Future<void> setupPin(String pin) async {
    await _secureStorage.write(key: _pinSetupKey, value: 'true');
  }

  String _generateRandomKey() {
    final random = Random.secure();
    final values = List<int>.generate(32, (i) => random.nextInt(256));
    return values.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
  }

  /// Attempts to start the Go engine using a key derived from the PIN.
  /// Returns the local API port if successful, or -1 on decryption error.
  Future<int> unlockAndStart(String pin, {int networkPort = 10001}) async {
    // 1. Get documents directory
    final Directory appDocDir = await getApplicationDocumentsDirectory();
    await appDocDir.create(recursive: true);
    final String dbPath = p.join(appDocDir.path, 'torbi.db');

    // Calculate PIN hash
    final pinBytes = utf8.encode(pin);
    final enteredPinHash = sha256.convert(pinBytes).toString();

    // Check secure storage for generated DB key & PIN hash
    String? storedDbPass = await _secureStorage.read(key: _dbKeyName);
    String? storedPinHash = await _secureStorage.read(key: _pinHashName);

    if (storedDbPass == null || storedPinHash == null) {
      // Clear any orphaned database/journal/WAL/shm/error files to prevent decryption failures
      for (final ext in ['', '-journal', '-wal', '-shm', '.init_err']) {
        final f = File('$dbPath$ext');
        if (await f.exists()) {
          try {
            await f.delete();
          } catch (_) {}
        }
      }

      // First-time setup: Generate database password and register PIN hash
      storedDbPass = _generateRandomKey();
      storedPinHash = enteredPinHash;

      await _secureStorage.write(key: _dbKeyName, value: storedDbPass);
      await _secureStorage.write(key: _pinHashName, value: storedPinHash);
    } else {
      // Subsequent launch: Verify PIN hash
      if (enteredPinHash != storedPinHash) {
        return -1; // Wrong PIN code
      }
    }

    // 3. Start Go runtime via FFI using the secure strong key
    final apiPort = TorbiBridge.startEngine(dbPath, storedDbPass, networkPort);
    
    if (apiPort > 0) {
      // Initialize the HTTP/WebSocket API client
      ApiClient().initialize(apiPort);
      
      // Save setup flag on success
      await setupPin(pin);
    } else {
      // Check for specific initialization error written by Go engine
      final errFile = File('$dbPath.init_err');
      if (await errFile.exists()) {
        final errText = await errFile.readAsString();
        try {
          await errFile.delete();
        } catch (_) {}
        throw Exception(errText);
      }
      throw Exception('Failed to start engine (port: $apiPort)');
    }
    
    return apiPort;
  }

  /// Stops the Go engine and releases database resources.
  Future<void> lock() async {
    TorbiBridge.stopEngine();
    ApiClient().close();
  }
}
