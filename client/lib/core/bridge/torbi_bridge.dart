import 'dart:ffi' as ffi;
import 'dart:io';
import 'package:ffi/ffi.dart';

typedef StartEngineC = ffi.Int32 Function(ffi.Pointer<Utf8> dbPath, ffi.Pointer<Utf8> dbPass, ffi.Int32 port);
typedef StartEngineDart = int Function(ffi.Pointer<Utf8> dbPath, ffi.Pointer<Utf8> dbPass, int port);

typedef StopEngineC = ffi.Void Function();
typedef StopEngineDart = void Function();

class TorbiBridge {
  static ffi.DynamicLibrary? _lib;

  static void load() {
    if (_lib != null) return;

    if (Platform.isWindows) {
      try {
        _lib = ffi.DynamicLibrary.open('torbi.dll');
      } catch (e) {
        // Fallback for custom run locations or testing
        _lib = ffi.DynamicLibrary.open('client/windows/torbi.dll');
      }
    } else if (Platform.isAndroid) {
      _lib = ffi.DynamicLibrary.open('libtorbi.so');
    } else if (Platform.isIOS || Platform.isMacOS) {
      _lib = ffi.DynamicLibrary.process();
    } else {
      throw UnsupportedError('Unsupported platform: ${Platform.operatingSystem}');
    }
  }

  /// Starts the Go core engine.
  /// Returns the HTTP loopback API port on success, or -1 on failure.
  static int startEngine(String dbPath, String dbPass, int port) {
    load();
    final startFn = _lib!.lookupFunction<StartEngineC, StartEngineDart>('start_engine');
    
    final dbPathPointer = dbPath.toNativeUtf8();
    final dbPassPointer = dbPass.toNativeUtf8();
    
    try {
      final allocatedPort = startFn(dbPathPointer, dbPassPointer, port);
      return allocatedPort;
    } finally {
      malloc.free(dbPathPointer);
      malloc.free(dbPassPointer);
    }
  }

  /// Stops the Go core engine.
  static void stopEngine() {
    load();
    final stopFn = _lib!.lookupFunction<StopEngineC, StopEngineDart>('stop_engine');
    stopFn();
  }
}
