export function normalizePath(path: string): string {
  return path.replace(/\\/g, '/').replace(/\/+/g, '/');
}

export function joinPaths(...paths: string[]): string {
  return normalizePath(paths.join('/'));
}

export function getFileName(path: string): string {
  const normalized = normalizePath(path);
  const parts = normalized.split('/');
  return parts[parts.length - 1] || '';
}

export function getDirectoryName(path: string): string {
  const normalized = normalizePath(path);
  const parts = normalized.split('/');
  parts.pop();
  return parts.join('/') || '.';
}

/**
 * Tronque un chemin pour l'affichage dans le graphe
 * - Garde les 2-3 derniers niveaux de dossier
 * - Limite à maxLength caractères
 */
export function truncatePathForDisplay(path: string, maxLength: number = 20, maxLevels: number = 3): string {
  if (!path) return '';

  const normalized = normalizePath(path);
  const parts = normalized.split('/');

  // Si le chemin est déjà court, le retourner tel quel
  if (normalized.length <= maxLength) {
    return normalized;
  }

  // Prendre les derniers niveaux
  const lastParts = parts.slice(-maxLevels);
  let truncated = lastParts.join('/');

  // Si encore trop long, tronquer avec ellipse
  if (truncated.length > maxLength) {
    const fileName = lastParts[lastParts.length - 1];
    if (fileName.length > maxLength - 3) {
      // Garder le début et la fin du nom de fichier
      const keepStart = Math.floor((maxLength - 3) / 2);
      const keepEnd = Math.ceil((maxLength - 3) / 2);
      return fileName.substring(0, keepStart) + '...' + fileName.substring(fileName.length - keepEnd);
    }
    return '.../' + fileName;
  }

  // Ajouter ... au début si on a tronqué des niveaux
  if (parts.length > maxLevels) {
    return '.../' + truncated;
  }

  return truncated;
}
