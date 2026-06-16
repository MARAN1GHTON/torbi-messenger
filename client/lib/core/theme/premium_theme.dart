import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

class PremiumTheme {
  // Brand Palette: Deep Obsidian, Space Grey, Neon Cyan, Electric Purple
  static const Color obsidianBg = Color(0xFF0B0E14);
  static const Color cardBg = Color(0xFF161B22);
  static const Color neonCyan = Color(0xFF00F0FF);
  static const Color electricPurple = Color(0xFF9D4EDD);
  static const Color textPrimary = Color(0xFFF0F6FC);
  static const Color textSecondary = Color(0xFF8B949E);
  static const Color successGreen = Color(0xFF3FB950);
  static const Color errorRed = Color(0xFFF85149);

  static ThemeData get darkTheme {
    return ThemeData(
      brightness: Brightness.dark,
      scaffoldBackgroundColor: obsidianBg,
      primaryColor: neonCyan,
      colorScheme: const ColorScheme.dark(
        primary: neonCyan,
        secondary: electricPurple,
        surface: cardBg,
        error: errorRed,
      ),
      textTheme: GoogleFonts.outfitTextTheme(
        const TextTheme(
          displayLarge: TextStyle(color: textPrimary, fontSize: 32, fontWeight: FontWeight.bold),
          titleLarge: TextStyle(color: textPrimary, fontSize: 20, fontWeight: FontWeight.w600),
          bodyLarge: TextStyle(color: textPrimary, fontSize: 16),
          bodyMedium: TextStyle(color: textSecondary, fontSize: 14),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: obsidianBg.withAlpha(128), // 0.5 opacity
        hintStyle: GoogleFonts.outfit(color: textSecondary),
        labelStyle: GoogleFonts.outfit(color: textSecondary),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: textSecondary.withAlpha(77), width: 1), // 0.3 opacity
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: neonCyan, width: 1.5),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: errorRed, width: 1),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: errorRed, width: 1.5),
        ),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: neonCyan,
          foregroundColor: obsidianBg,
          textStyle: GoogleFonts.outfit(fontWeight: FontWeight.bold, fontSize: 16),
          padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 24),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          elevation: 4,
          shadowColor: neonCyan.withAlpha(102), // 0.4 opacity
        ),
      ),
      cardTheme: CardThemeData(
        color: cardBg,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(16),
          side: BorderSide(color: textSecondary.withAlpha(26), width: 1), // 0.1 opacity
        ),
        elevation: 0,
      ),
    );
  }

  // Helper Glassmorphic Box Decoration
  static BoxDecoration glassDecoration() {
    return BoxDecoration(
      color: Colors.white.withAlpha(8), // 0.03 opacity
      borderRadius: BorderRadius.circular(16),
      border: Border.all(color: Colors.white.withAlpha(13), width: 1.5), // 0.05 opacity
    );
  }
}
