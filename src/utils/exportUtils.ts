import type { Core } from 'cytoscape';
import type { KustomizeGraph } from '../types/kustomize.types';

/**
 * Exporte le graphe Cytoscape en PNG (natif, pas de plugin requis)
 */
export const exportToPNG = (cy: Core, filename: string = 'kustomize-graph.png'): void => {
  const pngData = cy.png({
    scale: 2,
    full: true,
    bg: '#2c3e50'
  });
  
  const link = document.createElement('a');
  link.href = pngData;
  link.download = filename;
  link.click();
};

/**
 * Exporte le graphe au format Mermaid
 */
export const exportToMermaid = (graph: KustomizeGraph, filename: string = 'kustomize-graph.mmd'): void => {
  let mermaid = 'graph TB\n';
  
  // Nœuds
  for (const [, node] of graph.nodes) {
    const label = node.path || 'root';
    const nodeId = sanitizeId(node.id);
    
    if (node.type === 'resource') {
      mermaid += `  ${nodeId}[${label}]\n`;
    } else if (node.type === 'component') {
      mermaid += `  ${nodeId}[[${label}]]\n`;
    } else {
      mermaid += `  ${nodeId}(${label})\n`;
    }
  }
  
  // Arêtes
  for (const edge of graph.edges) {
    const source = sanitizeId(edge.source);
    const target = sanitizeId(edge.target);
    const label = edge.label || edge.type;
    
    if (edge.type === 'base') {
      mermaid += `  ${source} -.${label}.-> ${target}\n`;
    } else {
      mermaid += `  ${source} --${label}--> ${target}\n`;
    }
  }
  
  downloadFile(mermaid, filename, 'text/plain');
};

/**
 * Fonction helper pour télécharger un fichier
 */
const downloadFile = (content: string, filename: string, mimeType: string): void => {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

/**
 * Nettoie les IDs pour Mermaid
 */
const sanitizeId = (id: string): string => {
  return id.replace(/[^a-zA-Z0-9]/g, '_');
};

