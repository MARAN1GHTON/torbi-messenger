import 'dart:io';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../data/repositories/network_repository.dart';

// --- Events ---
abstract class SettingsEvent {}

class LoadSettings extends SettingsEvent {}

class ConnectToPeerEvent extends SettingsEvent {
  final String multiaddr;
  ConnectToPeerEvent(this.multiaddr);
}

// --- States ---
abstract class SettingsState {}

class SettingsInitial extends SettingsState {}

class SettingsLoading extends SettingsState {}

class SettingsLoaded extends SettingsState {
  final String peerId;
  final List<String> listenAddresses;
  final String natType;
  final int peersCount;
  final String? connectStatus; // E.g., "connecting", "success", "failed"

  SettingsLoaded({
    required this.peerId,
    required this.listenAddresses,
    required this.natType,
    required this.peersCount,
    this.connectStatus,
  });

  SettingsLoaded copyWith({
    String? connectStatus,
    int? peersCount,
  }) {
    return SettingsLoaded(
      peerId: peerId,
      listenAddresses: listenAddresses,
      natType: natType,
      peersCount: peersCount ?? this.peersCount,
      connectStatus: connectStatus ?? this.connectStatus,
    );
  }
}

class SettingsError extends SettingsState {
  final String message;
  SettingsError(this.message);
}

// --- Bloc ---
class SettingsBloc extends Bloc<SettingsEvent, SettingsState> {
  final NetworkRepository _networkRepository;

  SettingsBloc(this._networkRepository) : super(SettingsInitial()) {
    on<LoadSettings>((event, emit) async {
      emit(SettingsLoading());
      try {
        final status = await _networkRepository.getNodeStatus();
        final rawAddrs = (status['listen_addresses'] as List<dynamic>?)
            ?.map((e) => e.toString())
            .toList() ?? [];

        final List<String> addrs = [];
        final List<String> deviceIps = [];
        try {
          final interfaces = await NetworkInterface.list(
            includeLoopback: false,
            type: InternetAddressType.IPv4,
          );
          for (final interface in interfaces) {
            for (final addr in interface.addresses) {
              if (addr.address != '127.0.0.1' && !addr.address.startsWith('169.254')) {
                deviceIps.add(addr.address);
              }
            }
          }
        } catch (_) {}

        for (final rawAddr in rawAddrs) {
          if (rawAddr.contains('/ip4/127.0.0.1/')) {
            addrs.add(rawAddr); // Keep loopback
            for (final ip in deviceIps) {
              addrs.add(rawAddr.replaceAll('127.0.0.1', ip));
            }
          } else {
            addrs.add(rawAddr);
          }
        }

        emit(SettingsLoaded(
          peerId: status['peer_id'] as String? ?? 'Unknown',
          listenAddresses: addrs,
          natType: status['nat_type'] as String? ?? 'Unknown',
          peersCount: status['peers_count'] as int? ?? 0,
        ));
      } catch (e) {
        emit(SettingsError('Failed to load settings data: $e'));
      }
    });

    on<ConnectToPeerEvent>((event, emit) async {
      if (state is SettingsLoaded) {
        final current = state as SettingsLoaded;
        emit(current.copyWith(connectStatus: 'connecting'));
        try {
          await _networkRepository.connectToMultiaddress(event.multiaddr);
          
          // Fetch updated status to reflect new connection count
          final status = await _networkRepository.getNodeStatus();
          
          emit(current.copyWith(
            connectStatus: 'success',
            peersCount: status['peers_count'] as int? ?? 0,
          ));
        } catch (e) {
          emit(current.copyWith(connectStatus: 'failed: $e'));
        }
      }
    });
  }
}
