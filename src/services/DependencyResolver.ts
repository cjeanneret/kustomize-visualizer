import type {
  KustomizeNode,
  KustomizeGraph,
  DependencyEdge
} from '../types/kustomize.types';

export class DependencyResolver {
  private edgeCounter = 0;

  buildGraph(nodes: KustomizeNode[]): KustomizeGraph {
    const nodeMap = new Map<string, KustomizeNode>();
    const edges: DependencyEdge[] = [];

    // Indexer les nœuds par chemin
    for (const node of nodes) {
      nodeMap.set(node.path, node);
    }

    console.log(`\n🔗 Construction du graphe de dépendances...`);
    console.log(`📊 ${nodes.length} nœuds à analyser`);

    // Construire les arêtes
    for (const node of nodes) {
      this.buildEdgesForNode(node, nodeMap, edges);
    }

    console.log(`✓ ${edges.length} arête(s) créée(s)`);

    // Corriger les types basés sur comment ils sont référencés
    this.correctNodeTypes(nodeMap, edges);

    return {
      nodes: nodeMap,
      edges,
      rootPath: nodes[0]?.path || ''
    };
  }

  /**
   * Corrige les types de nœuds selon comment ils sont référencés
   * RÈGLE SIMPLE : component si dans components:, resource sinon
   */
  private correctNodeTypes(
    nodeMap: Map<string, KustomizeNode>,
    edges: DependencyEdge[]
  ): void {
    console.log('\n🔄 Correction des types de nœuds...');

    // Collecter tous les nœuds référencés comme components
    const componentNodeIds = new Set<string>();

    for (const edge of edges) {
      if (edge.type === 'component') {
        componentNodeIds.add(edge.target);
      }
    }

    // Appliquer les types
    for (const node of nodeMap.values()) {
      const oldType = node.type;

      if (componentNodeIds.has(node.id)) {
        node.type = 'component';
      } else {
        node.type = 'resource';
      }

      if (oldType !== node.type) {
        console.log(`  📝 ${node.path}: ${oldType} → ${node.type}`);
      }
    }

    console.log(`✓ Types corrigés: ${componentNodeIds.size} components, ${nodeMap.size - componentNodeIds.size} resources`);
  }

  private buildEdgesForNode(
    node: KustomizeNode,
    nodeMap: Map<string, KustomizeNode>,
    edges: DependencyEdge[]
  ): void {
    const kustomization = node.kustomizationContent;
    console.log(`\n  🔍 Analyse du nœud: ${node.path}`);

    // Traiter resources
    if (kustomization.resources && kustomization.resources.length > 0) {
      console.log(`    📦 Resources: ${kustomization.resources.length}`);
      for (const resource of kustomization.resources) {
        // Ignorer les fichiers YAML simples
        if (!resource.endsWith('.yaml') && !resource.endsWith('.yml')) {
          this.processReference(node, resource, 'resource', nodeMap, edges);
        } else {
          console.log(`    ℹ️ Ignoré (fichier YAML): ${resource}`);
        }
      }
    }

    // Traiter bases (déprécié) - les traiter comme des resources
    if (kustomization.bases && kustomization.bases.length > 0) {
      console.log(`    📦 Bases (déprécié): ${kustomization.bases.length}`);
      for (const base of kustomization.bases) {
        this.processReference(node, base, 'resource', nodeMap, edges);
      }
    }

    // Traiter components
    if (kustomization.components && kustomization.components.length > 0) {
      console.log(`    📦 Components: ${kustomization.components.length}`);
      for (const component of kustomization.components) {
        this.processReference(node, component, 'component', nodeMap, edges);
      }
    }
  }

  private processReference(
    sourceNode: KustomizeNode,
    reference: string,
    type: 'resource' | 'component',
    nodeMap: Map<string, KustomizeNode>,
    edges: DependencyEdge[]
  ): void {
    console.log(`      → ${type}: ${reference}`);

    if (this.isRemoteUrl(reference)) {
      // C'est une URL distante
      console.log(`        ℹ️ URL distante détectée`);

      const remoteNodeId = `remote-${this.edgeCounter}`;
      const remoteDisplayName = this.extractDisplayNameFromUrl(reference);

      let targetNodeId = remoteNodeId;

      // Chercher si un nœud existe déjà avec cette URL
      for (const [, node] of nodeMap) {
        if (node.remoteUrl === reference) {
          targetNodeId = node.id;
          console.log(`        ✓ Nœud existant trouvé: ${node.path}`);
          break;
        }
      }

      // Si pas de nœud existant, en créer un virtuel
      if (targetNodeId === remoteNodeId) {
        const virtualNode: KustomizeNode = {
          id: remoteNodeId,
          path: remoteDisplayName,
          type: type,  // component ou resource selon le contexte
          kustomizationContent: {},
          isRemote: true,
          remoteUrl: reference,
          loaded: false
        };
        nodeMap.set(virtualNode.path, virtualNode);
        console.log(`        + Nœud virtuel créé: ${remoteDisplayName}`);
      }

      edges.push({
        id: `edge-${this.edgeCounter++}`,
        source: sourceNode.id,
        target: targetNodeId,
        type,
        label: this.extractLabelFromUrl(reference)
      });
      console.log(`        ✓ Arête créée`);
    } else if (this.isLocalPath(reference)) {
      // C'est un chemin local relatif
      const resolvedPath = this.resolvePath(sourceNode.path, reference);
      console.log(`        📂 Chemin local: ${reference} → ${resolvedPath}`);

      const targetNode = nodeMap.get(resolvedPath);
      if (targetNode) {
        edges.push({
          id: `edge-${this.edgeCounter++}`,
          source: sourceNode.id,
          target: targetNode.id,
          type,
          label: reference
        });
        console.log(`        ✓ Arête créée vers: ${targetNode.path}`);
      } else {
        console.log(`        ⚠️ Nœud cible non trouvé: ${resolvedPath}`);

        // Créer un nœud "manquant"
        const missingNodeId = `missing-${this.edgeCounter}`;
        const missingNode: KustomizeNode = {
          id: missingNodeId,
          path: resolvedPath,
          type: 'resource',
          kustomizationContent: {},
          isRemote: false,
          loaded: false
        };
        nodeMap.set(missingNode.path, missingNode);

        edges.push({
          id: `edge-${this.edgeCounter++}`,
          source: sourceNode.id,
          target: missingNodeId,
          type,
          label: reference
        });
        console.log(`        + Nœud "manquant" créé`);
      }
    }
  }

  private isRemoteUrl(path: string): boolean {
    return path.startsWith('http://') || path.startsWith('https://');
  }

  private isLocalPath(path: string): boolean {
    return !this.isRemoteUrl(path);
  }

  private extractDisplayNameFromUrl(url: string): string {
    try {
      const cleanUrl = url.split('?')[0];
      const match = cleanUrl.match(/github\.com\/[^\/]+\/[^\/]+\/(.+)/);
      if (match) {
        return match[1];
      }
      const parts = cleanUrl.split('/');
      return parts.slice(-2).join('/');
    } catch {
      return url;
    }
  }

  private extractLabelFromUrl(url: string): string {
    try {
      const parts = url.split('/');
      const lastPart = parts[parts.length - 1].split('?')[0];
      return lastPart || 'remote';
    } catch {
      return 'remote';
    }
  }

  private resolvePath(basePath: string, relativePath: string): string {
    const parts = basePath === '.' ? [] : basePath.split('/').filter(p => p !== '');
    const relParts = relativePath.split('/').filter(p => p !== '');

    for (const part of relParts) {
      if (part === '..') {
        parts.pop();
      } else if (part !== '.') {
        parts.push(part);
      }
    }

    const result = parts.join('/') || '.';
    return result;
  }

  detectCycles(graph: KustomizeGraph): string[][] {
    const cycles: string[][] = [];
    const visited = new Set<string>();
    const recStack = new Set<string>();

    const dfs = (nodeId: string, path: string[]): void => {
      visited.add(nodeId);
      recStack.add(nodeId);
      path.push(nodeId);

      const outEdges = graph.edges.filter(e => e.source === nodeId);

      for (const edge of outEdges) {
        if (!visited.has(edge.target)) {
          dfs(edge.target, [...path]);
        } else if (recStack.has(edge.target)) {
          const cycleStart = path.indexOf(edge.target);
          cycles.push([...path.slice(cycleStart), edge.target]);
        }
      }

      recStack.delete(nodeId);
    };

    for (const [, node] of graph.nodes) {
      if (!visited.has(node.id)) {
        dfs(node.id, []);
      }
    }

    return cycles;
  }
}

