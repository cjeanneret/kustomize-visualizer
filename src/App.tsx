import React, { useState } from 'react';
import { CollapsibleSidebar } from './components/Sidebar/CollapsibleSidebar';
import { GraphCanvas } from './components/GraphCanvas/GraphCanvas';
import { CollapsibleDetailsPanel } from './components/DetailsPanel/CollapsibleDetailsPanel';
import { GitCrawler } from './services/GitCrawler';
import { DependencyResolver } from './services/DependencyResolver';
import type { KustomizeGraph, KustomizeNode } from './types/kustomize.types';
import './App.css';

function App() {
  const [graph, setGraph] = useState<KustomizeGraph | null>(null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);

  const handleLoadRepo = async (
    source: string,
    isLocal: boolean,
    githubToken?: string,
    gitlabToken?: string
  ) => {
    console.log('🚀 Démarrage du chargement...');

    const crawler = new GitCrawler();

    // Configurer les tokens si fournis
    if (githubToken) {
      crawler.setGitHubToken(githubToken);
    }
    if (gitlabToken) {
      crawler.setGitLabToken(gitlabToken);
    }

    let nodes;
    if (isLocal) {
      nodes = await crawler.scanLocalDirectory();
    } else {
      nodes = await crawler.crawlFromOverlay(source);
    }

    console.log(`✓ ${nodes.length} nœud(s) trouvé(s)`);

    if (nodes.length === 0) {
      throw new Error('Aucun fichier kustomization.yaml trouvé');
    }

    const resolver = new DependencyResolver();
    const newGraph = resolver.buildGraph(nodes);

    console.log(`✓ Graphe: ${newGraph.nodes.size} nœuds, ${newGraph.edges.length} arêtes`);

    const cycles = resolver.detectCycles(newGraph);
    if (cycles.length > 0) {
      console.warn('⚠️ Cycles détectés:', cycles);
    }

    setGraph(newGraph);
    setSelectedNodeId(null);
  };

  const selectedNode: KustomizeNode | null = selectedNodeId && graph
    ? Array.from(graph.nodes.values()).find(n => n.id === selectedNodeId) || null
    : null;

  return (
    <div className="app">
      <CollapsibleSidebar onLoadRepo={handleLoadRepo} />
      <GraphCanvas graph={graph} onNodeSelect={setSelectedNodeId} />
      <CollapsibleDetailsPanel node={selectedNode} />
    </div>
  );
}

export default App;

