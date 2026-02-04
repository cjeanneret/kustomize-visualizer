import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import cytoscape, { Core, ElementDefinition } from 'cytoscape';
import dagre from 'cytoscape-dagre';
import type { KustomizeGraph } from '../../types/kustomize.types';
import { GraphLegend } from './GraphLegend';
import { truncatePathForDisplay } from '../../utils/pathUtils';
import { exportToPNG, exportToMermaid } from '../../utils/exportUtils';

import './GraphCanvas.css';

cytoscape.use(dagre);

interface GraphCanvasProps {
    graph: KustomizeGraph | null;
    onNodeSelect: (nodeId: string) => void;
}

export const GraphCanvas: React.FC<GraphCanvasProps> = ({ graph, onNodeSelect }) => {
    const { t } = useTranslation();
    const cyContainerRef = useRef<HTMLDivElement>(null);
    const cyRef = useRef<Core | null>(null);
    const [isReady, setIsReady] = useState(false);

    // Initialiser Cytoscape une seule fois
    useLayoutEffect(() => {
        if (!cyContainerRef.current) {
            console.error('❌ cyContainerRef.current est null');
            return;
        }

        if (cyRef.current) {
            return;
        }

        console.log('🎨 Initialisation de Cytoscape...');

        try {
            cyRef.current = cytoscape({
                container: cyContainerRef.current,
                style: [
                    {
                        selector: 'node',
                        style: {
                            'background-color': '#3498db',
                            'label': 'data(label)',
                            'color': '#ffffff',
                            'text-valign': 'center',
                            'text-halign': 'center',
                            'font-size': '10px',
                            'font-weight': 'bold',
                            'width': '120px',
                            'height': '120px',
                            'border-width': 4,
                            'border-color': '#ffffff',
                            'text-wrap': 'wrap',
                            'text-max-width': '110px',
                            'text-background-color': '#2c3e50',
                            'text-background-opacity': 0.9,
                            'text-background-padding': '4px',
                            'text-background-shape': 'roundrectangle'
                        }
                    },
                    {
                        selector: 'node[type="resource"]',
                        style: {
                            'background-color': '#2ecc71',
                            'shape': 'rectangle'
                        }
                    },
                    {
                        selector: 'node[type="component"]',
                        style: {
                            'background-color': '#e67e22',
                            'shape': 'roundrectangle'
                        }
                    },
                    {
                        selector: 'node[loaded="false"]',
                        style: {
                            'border-style': 'dashed',
                            'border-width': 4,
                            'opacity': 0.75,
                            'background-opacity': 0.6
                        }
                    },
                    {
                        selector: 'edge',
                        style: {
                            'width': 3,
                            'line-color': '#95a5a6',
                            'target-arrow-color': '#95a5a6',
                            'target-arrow-shape': 'triangle',
                            'curve-style': 'bezier',
                            'label': 'data(label)',
                            'font-size': '9px',
                            'color': '#ecf0f1',
                            'text-rotation': 'autorotate',
                            'text-margin-y': -12,
                            'text-background-color': '#2c3e50',
                            'text-background-opacity': 0.8,
                            'text-background-padding': '2px'
                        }
                    },
                    {
                        selector: 'edge[type="resource"]',
                        style: {
                            'line-color': '#3498db',
                            'target-arrow-color': '#3498db',
                            'width': 3
                        }
                    },
                    {
                        selector: 'edge[type="base"]',
                        style: {
                            'line-color': '#e74c3c',
                            'target-arrow-color': '#e74c3c',
                            'line-style': 'dashed',
                            'width': 3
                        }
                    },
                    {
                        selector: 'edge[type="component"]',
                        style: {
                            'line-color': '#f39c12',
                            'target-arrow-color': '#f39c12',
                            'width': 4
                        }
                    },
                    {
                        selector: ':selected',
                        style: {
                            'background-color': '#e74c3c',
                            'line-color': '#e74c3c',
                            'target-arrow-color': '#e74c3c',
                            'border-width': 6,
                            'border-color': '#c0392b'
                        }
                    }
                ]
            });

            console.log('✅ Cytoscape initialisé');
            setIsReady(true);

        } catch (error) {
            console.error('❌ Erreur initialisation:', error);
        }

        return () => {
            if (cyRef.current) {
                try {
                    cyRef.current.destroy();
                } catch (e) {
                    console.error('Erreur destruction:', e);
                }
                cyRef.current = null;
            }
            setIsReady(false);
        };
    }, []);

    // Gérer la sélection
    useEffect(() => {
        if (!cyRef.current || !isReady) return;

        const handleTap = (evt: any) => {
            onNodeSelect(evt.target.id());
        };

        cyRef.current.on('tap', 'node', handleTap);

        return () => {
            if (cyRef.current) {
                cyRef.current.off('tap', 'node', handleTap);
            }
        };
    }, [onNodeSelect, isReady]);

    // Mettre à jour le graphe
    useEffect(() => {
        if (!isReady || !cyRef.current || !graph) {
            return;
        }

        console.log('🎨 Mise à jour du graphe...');

        const elements: ElementDefinition[] = [];

        for (const [, node] of graph.nodes) {
            elements.push({
                data: {
                    id: node.id,
                    label: truncatePathForDisplay(node.path || 'root', 30, 2),
                    type: node.type,
                    loaded: node.loaded ? 'true' : 'false'
                }
            });
        }

        for (const edge of graph.edges) {
            elements.push({
                data: {
                    id: edge.id,
                    source: edge.source,
                    target: edge.target,
                    label: edge.label || '',
                    type: edge.type
                }
            });
        }

        try {
            cyRef.current.elements().remove();
            cyRef.current.add(elements);

            const layout = cyRef.current.layout({
                name: 'dagre',
                rankDir: 'TB',
                nodeSep: 100,
                rankSep: 150,
                padding: 50,
                animate: false
            } as any);

            layout.run();

            setTimeout(() => {
                if (cyRef.current) {
                    cyRef.current.fit(undefined, 50);
                    console.log('✅ Graphe affiché');
                }
            }, 100);

        } catch (error) {
            console.error('❌ Erreur mise à jour:', error);
        }

    }, [graph, isReady]);

    // TOUJOURS rendre le conteneur Cytoscape
    return (
        <div className={graph ? "graph-canvas" : "graph-canvas graph-empty"}>
        {/* Conteneur Cytoscape - toujours présent */}
        <div
        ref={cyContainerRef}
        style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            width: '100%',
            height: '100%'
        }}
        />

        {/* Message si pas de graphe */}
        {!graph && (
            <div className="empty-state">
            <h2>{t('graph.noGraphTitle')}</h2>
            <p>{t('graph.noGraphDescription')}</p>
            </div>
        )}

        {/* Overlays si graphe présent */}
        {graph && (
            <>
            <GraphLegend />
            {/* Boutons d'export */}
            <div style={{
                position: 'absolute',
                top: '10px',
                left: '10px',
                display: 'flex',
                gap: '10px',
                zIndex: 1000
            }}>
            <button 
            onClick={() => cyRef.current && exportToPNG(cyRef.current)}
            style={{
                padding: '8px 12px',
                background: '#3498db',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer'
            }}
            >
            📥 PNG
            </button>

            <button 
            onClick={() => graph && exportToMermaid(graph)}
            style={{
                padding: '8px 12px',
                background: '#9b59b6',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer'
            }}
            >
            📥 Mermaid
            </button>
            </div>

            <div style={{
                position: 'absolute',
                top: '10px',
                right: '10px',
                background: 'rgba(0,0,0,0.8)',
                color: 'white',
                padding: '10px',
                borderRadius: '5px',
                fontSize: '12px',
                zIndex: 1000,
                pointerEvents: 'none'
            }}>
            <div>{t('graph.nodesLabel', { count: graph.nodes.size })}</div>
            <div>{t('graph.edgesLabel', { count: graph.edges.length })}</div>
            <div>{isReady ? t('graph.statusReady') : t('graph.statusInit')}</div>
            </div>
            </>
        )}
        </div>
    );
};
