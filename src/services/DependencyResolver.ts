import type {
    KustomizeNode,
    KustomizeGraph,
    DependencyEdge
} from '../types/kustomize.types';

export class DependencyResolver {
    private edgeCounter = 0;

    /**
     * Construire le graphe à partir des nœuds crawlés
     * La logique est simplifiée car les types sont déjà corrects depuis le crawler
     */
    buildGraph(nodes: KustomizeNode[]): KustomizeGraph {
        const nodeMap = new Map<string, KustomizeNode>();
        const edges: DependencyEdge[] = [];

        console.log(`\n🔗 Construction du graphe de dépendances...`);
        console.log(`📊 ${nodes.length} nœuds à analyser`);

        // Indexer tous les nœuds
        for (const node of nodes) {
            nodeMap.set(node.path, node);
        }

        console.log(`✓ ${nodeMap.size} nœuds indexés`);

        // Construire les arêtes
        for (const node of nodes) {
            this.buildEdgesForNode(node, nodes, edges);
        }

        console.log(`✓ ${edges.length} arête(s) créée(s)`);

        return {
            nodes: nodeMap,
            edges,
            rootPath: nodes[0]?.path || ''
        };
    }

    /**
     * Construire les arêtes pour un nœud
     */
    private buildEdgesForNode(
        sourceNode: KustomizeNode,
        allNodes: KustomizeNode[],
        edges: DependencyEdge[]
    ): void {
        const kustomization = sourceNode.kustomizationContent;

        // Traiter les resources
        if (kustomization.resources && kustomization.resources.length > 0) {
            for (const resource of kustomization.resources) {
                this.createEdgeIfTargetExists(
                    sourceNode,
                    resource,
                    'resource',
                    allNodes,
                    edges
                );
            }
        }

        // Traiter les bases (déprécié)
        if (kustomization.bases && kustomization.bases.length > 0) {
            for (const base of kustomization.bases) {
                this.createEdgeIfTargetExists(
                    sourceNode,
                    base,
                    'resource',
                    allNodes,
                    edges
                );
            }
        }

        // Traiter les components
        if (kustomization.components && kustomization.components.length > 0) {
            for (const component of kustomization.components) {
                this.createEdgeIfTargetExists(
                    sourceNode,
                    component,
                    'component',
                    allNodes,
                    edges
                );
            }
        }
    }

    /**
     * Créer une arête si le nœud cible existe
     */
    private createEdgeIfTargetExists(
        sourceNode: KustomizeNode,
        reference: string,
        edgeType: 'resource' | 'component',
        allNodes: KustomizeNode[],
        edges: DependencyEdge[]
    ): void {
        // Chercher le nœud cible
        let targetNode: KustomizeNode | undefined;

        // Cas 1: référence distante (URL complète)
        if (this.isRemoteUrl(reference)) {
            targetNode = allNodes.find(n => n.remoteUrl === reference);
        }
        // Cas 2: référence locale (chemin relatif)
        else {
            const resolvedPath = this.resolvePath(sourceNode.path, reference);
            targetNode = allNodes.find(n => {
                const normalizedNodePath = n.path.replace(/^\.\//, '').replace(/\/$/, '');
                const normalizedResolvedPath = resolvedPath.replace(/^\.\//, '').replace(/\/$/, '');
                return normalizedNodePath === normalizedResolvedPath;
            });
        }

        // Si le nœud cible existe, créer l'arête
        if (targetNode) {
            edges.push({
                id: `edge-${this.edgeCounter++}`,
                source: sourceNode.id,
                target: targetNode.id,
                type: edgeType,
                label: this.extractLabel(reference)
            });
        }
    }

    /**
     * Vérifier si c'est une URL distante
     */
    private isRemoteUrl(path: string): boolean {
        return path.startsWith('http://') || path.startsWith('https://');
    }

    /**
     * Résoudre un chemin relatif
     */
    private resolvePath(basePath: string, relativePath: string): string {
        const cleanBase = basePath.replace(/^\.\//, '').replace(/\/$/, '');
        const cleanRel = relativePath.replace(/^\.\//, '').replace(/\/$/, '');

        const parts = cleanBase === '.' || cleanBase === '' ? [] : cleanBase.split('/').filter(p => p !== '');
        const relParts = cleanRel.split('/').filter(p => p !== '');

        for (const part of relParts) {
            if (part === '..') {
                parts.pop();
            } else if (part !== '.' && part !== '') {
                parts.push(part);
            }
        }

        return parts.join('/') || '.';
    }

    /**
     * Extraire un label depuis une référence
     */
    private extractLabel(reference: string): string {
        if (this.isRemoteUrl(reference)) {
            try {
                const parts = reference.split('/');
                const lastPart = parts[parts.length - 1].split('?')[0];
                return lastPart || 'remote';
            } catch {
                return 'remote';
            }
        }
        return reference;
    }

    /**
     * Détecter les cycles dans le graphe
     */
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
