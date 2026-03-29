package dashboard

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>ToT Dashboard</title>
<script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/4.4.1/chart.umd.js"></script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
:root{--bg:#fff;--bg2:#f5f4f0;--bg3:#eae9e5;--tx:#1a1a1a;--tx2:#6b6a65;--tx3:#9c9a92;--bd:#dddcd7;--blue:#378ADD;--green:#639922;--amber:#BA7517;--red:#E24B4A;--gray:#888780;--purple:#7F77DD;--teal:#1D9E75;--radius:8px;--mono:'SF Mono',Consolas,'Liberation Mono',Menlo,monospace;--sans:system-ui,-apple-system,sans-serif}
@media(prefers-color-scheme:dark){:root{--bg:#1a1a1a;--bg2:#2a2a28;--bg3:#333331;--tx:#e8e6de;--tx2:#9c9a92;--tx3:#6b6a65;--bd:#444441}}
body{font-family:var(--sans);background:var(--bg);color:var(--tx);max-width:900px;margin:0 auto;padding:24px 20px}
a{color:var(--blue);text-decoration:none}
h1{font-size:18px;font-weight:500;margin-bottom:4px}
.subtitle{font-size:13px;color:var(--tx2);margin-bottom:20px}
.grid{display:grid;gap:12px}
.grid-2{grid-template-columns:1fr 1fr}
.grid-4{grid-template-columns:repeat(4,1fr)}
@media(max-width:640px){.grid-4{grid-template-columns:1fr 1fr}.grid-2{grid-template-columns:1fr}}
.card{background:var(--bg2);border-radius:var(--radius);padding:12px 16px}
.card-border{background:var(--bg);border:0.5px solid var(--bd);border-radius:10px;padding:16px 20px;margin-bottom:12px}
.metric-label{font-size:12px;color:var(--tx2)}
.metric-val{font-size:22px;font-weight:500;margin:2px 0}
.metric-sub{font-size:11px;color:var(--tx3)}
.section{font-size:11px;font-weight:500;letter-spacing:0.5px;color:var(--tx3);text-transform:uppercase;margin:20px 0 8px}
.tree-list-item{display:flex;justify-content:space-between;align-items:center;padding:12px 16px;border:0.5px solid var(--bd);border-radius:var(--radius);margin-bottom:8px;cursor:pointer;transition:background .15s}
.tree-list-item:hover{background:var(--bg2)}
.tree-problem{font-size:14px;font-weight:500}
.tree-meta{font-size:12px;color:var(--tx2);display:flex;gap:12px;margin-top:4px}
.tag{font-size:11px;padding:2px 8px;border-radius:var(--radius)}
.tag-keep{background:#eaf3de;color:#3b6d11}
.tag-discard{background:var(--bg3);color:var(--tx2)}
.tag-crash{background:#fcebeb;color:#a32d2d}
@media(prefers-color-scheme:dark){.tag-keep{background:#27500a;color:#97c459}.tag-crash{background:#501313;color:#f09595}}
.exp-row{display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:0.5px solid var(--bd);font-size:13px}
.exp-row:last-child{border-bottom:none}
.dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
.dot-improved{background:var(--green)}
.dot-regressed{background:var(--gray)}
.dot-crashed{background:var(--red)}
.dot-timeout{background:var(--amber)}
.exp-label{flex:1}
.exp-metric{font-family:var(--mono);font-size:12px;font-weight:500;min-width:60px;text-align:right}
.back{font-size:13px;color:var(--tx2);cursor:pointer;margin-bottom:16px;display:inline-block}
.back:hover{color:var(--tx)}
.sol-card{padding:14px 16px;border:0.5px solid var(--bd);border-radius:var(--radius);margin-bottom:10px}
.sol-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:8px}
.sol-problem{font-size:14px;font-weight:600}
.sol-badge{font-size:11px;font-family:var(--mono);padding:2px 8px;border-radius:var(--radius);background:var(--bg2);color:var(--tx2)}
.sol-text{font-size:13px;color:var(--tx);line-height:1.6;white-space:pre-wrap;word-break:break-word}
.sol-tags{margin-top:8px;display:flex;flex-wrap:wrap;gap:4px}
.sol-tag{font-size:10px;padding:2px 6px;border-radius:4px;background:var(--bg3);color:var(--tx2)}
.path-card{border:0.5px solid var(--bd);border-radius:var(--radius);margin-bottom:10px;overflow:hidden}
.path-header{display:flex;justify-content:space-between;align-items:center;padding:12px 16px;cursor:pointer;transition:background .15s}
.path-header:hover{background:var(--bg2)}
.path-rank{font-size:13px;font-weight:600;min-width:32px}
.path-summary{flex:1;font-size:13px;margin:0 12px}
.path-score{font-family:var(--mono);font-size:12px;padding:2px 8px;border-radius:var(--radius);background:var(--bg2);color:var(--tx2)}
.path-body{display:none;padding:0 16px 16px}
.path-body.open{display:block}
.path-step{position:relative;padding:12px 16px 12px 32px;margin-bottom:0}
.path-step:not(:last-child){border-left:2px solid var(--bd);margin-left:8px}
.path-step:last-child{border-left:2px solid var(--green);margin-left:8px}
.path-depth{position:absolute;left:-6px;top:12px;width:14px;height:14px;border-radius:50%;border:2px solid var(--bd);background:var(--bg);font-size:0}
.path-step:last-child .path-depth{border-color:var(--green);background:var(--green)}
.path-step-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:4px}
.path-step-label{font-size:11px;font-weight:600;color:var(--tx2);text-transform:uppercase;letter-spacing:0.3px}
.path-step-score{font-family:var(--mono);font-size:11px;color:var(--tx2)}
.path-step-text{font-size:13px;line-height:1.6;color:var(--tx);white-space:pre-wrap;word-break:break-word}
.tree-svg{width:100%;overflow-x:auto}
.node-rect{cursor:pointer;transition:opacity .15s}
.node-rect:hover{opacity:0.85}
.legend{display:flex;gap:14px;margin-bottom:10px;font-size:12px;color:var(--tx2);flex-wrap:wrap}
.legend span{display:flex;align-items:center;gap:4px}
.legend i{width:10px;height:10px;border-radius:2px;display:inline-block}
.chart-wrap{position:relative;height:220px;margin-top:8px}
#app{min-height:300px}
.empty{text-align:center;padding:60px 20px;color:var(--tx2);font-size:14px}
</style>
</head>
<body>
<div id="app"></div>
<script>
const app = document.getElementById('app');
let currentView = 'list';
let currentTreeID = null;

const COLORS = {sure:'#378ADD',maybe:'#BA7517',impossible:'#E24B4A',solution:'#639922',unexplored:'#888780'};

async function fetchJSON(url) {
  const r = await fetch(url);
  return r.json();
}

function render() {
  const path = location.hash.replace('#','');
  if (path.startsWith('tree/')) {
    currentTreeID = path.replace('tree/','');
    renderTreeDetail();
  } else {
    renderTreeList();
  }
}

async function renderTreeList() {
  const trees = await fetchJSON('/api/trees');
  if (!trees || trees.length === 0) {
    app.innerHTML = '<div class="empty"><h1>Tree of Thoughts</h1><p style="margin-top:8px">No reasoning trees yet. Use the MCP tools to create one.</p></div>';
    return;
  }
  let html = '<h1>Tree of Thoughts</h1><p class="subtitle">All reasoning trees</p>';
  for (const t of trees) {
    html += '<div class="tree-list-item" onclick="location.hash=\'tree/'+t.id+'\'">';
    html += '<div><div class="tree-problem">'+esc(t.problem)+'</div>';
    html += '<div class="tree-meta"><span>'+t.nodeCount+' nodes</span><span>'+t.experimentCount+' experiments</span><span>'+t.strategy+'</span></div></div>';
    html += '<span class="tag '+(t.status==='active'?'tag-keep':'tag-discard')+'">'+t.status+'</span>';
    html += '</div>';
  }
  app.innerHTML = html;
}

async function renderTreeDetail() {
  const [treeData, expData, retData] = await Promise.all([
    fetchJSON('/api/tree/'+currentTreeID),
    fetchJSON('/api/experiments/'+currentTreeID),
    fetchJSON('/api/retrieval/'+currentTreeID),
  ]);

  const t = treeData.tree;
  const nodes = treeData.nodes || [];
  const stats = treeData.stats;
  const bestPath = new Set(treeData.bestPath || []);
  const exps = expData.experiments || [];
  const expStats = expData.stats;
  const sols = retData.solutions || [];

  let html = '<span class="back" onclick="location.hash=\'\'">&larr; all trees</span>';
  html += '<h1>'+esc(t.problem)+'</h1>';
  html += '<p class="subtitle">'+t.strategy+' search &middot; max depth '+t.maxDepth+' &middot; branching '+t.branchingFactor+'</p>';

  // Metric cards
  const bestNode = nodes.filter(n=>n.isTerminal||bestPath.has(n.id)).sort((a,b)=>b.score-a.score)[0];
  const bestMetric = exps.filter(e=>e.metric).sort((a,b)=>{
    return a.metric-b.metric;
  })[0];

  if (expStats.total > 0) {
    html += '<div class="grid grid-4">';
    html += metricCard('Nodes', stats.total, stats.pruned+' pruned, '+stats.terminal+' solutions');
    html += metricCard('Experiments', expStats.total, expStats.improved+' kept, '+expStats.crashed+' crashed');
    html += metricCard('Success rate', expStats.successRate+'%', expStats.discarded+' discarded');
    html += metricCard('Best metric', bestMetric?bestMetric.metric.toFixed(4):'N/A', bestMetric?esc(getNodeThought(nodes, bestMetric.nodeId)):'no experiments');
    html += '</div>';
  } else {
    html += '<div class="grid grid-4">';
    html += metricCard('Nodes', stats.total, stats.pruned+' pruned, '+stats.terminal+' solutions');
    html += metricCard('Frontier', stats.frontier, 'nodes to expand');
    html += metricCard('Max depth', stats.maxDepth, 'of '+treeData.tree.maxDepth+' limit');
    html += metricCard('Active', stats.active, stats.total+' total nodes');
    html += '</div>';
  }

  // Tree visualization
  html += '<div class="section">Reasoning tree</div>';
  html += '<div class="card-border">';
  html += '<div class="legend">';
  html += '<span><i style="background:'+COLORS.sure+'"></i> Sure</span>';
  html += '<span><i style="background:'+COLORS.maybe+'"></i> Maybe</span>';
  html += '<span><i style="background:'+COLORS.impossible+'"></i> Impossible</span>';
  html += '<span><i style="background:'+COLORS.solution+'"></i> Solution</span>';
  html += '<span><i style="background:'+COLORS.unexplored+'"></i> Unexplored</span>';
  html += '</div>';
  html += buildTreeSVG(nodes, bestPath);
  html += '</div>';

  // Experiments + Retrieval side by side (or just retrieval if no experiments)
  const showExps = exps.length > 0;
  const showSols = sols.length > 0;
  if (showExps || showSols) {
    html += showExps ? '<div class="grid grid-2">' : '<div>';

    // Experiments — only when they exist
    if (showExps) {
      html += '<div class="card-border">';
      html += '<div style="font-size:14px;font-weight:500;margin-bottom:10px">Experiment history</div>';
      for (const e of exps) {
        const dotClass = 'dot dot-'+(e.status||'regressed');
        const label = getNodeThought(nodes, e.nodeId);
        const metricStr = e.metric != null ? e.metric.toFixed(4) : (e.status==='crashed'?'crash':'N/A');
        const tagClass = e.kept?'tag-keep':(e.status==='crashed'?'tag-crash':'tag-discard');
        const tagLabel = e.kept?'keep':(e.status==='crashed'?'crash':'discard');
        html += '<div class="exp-row"><span class="'+dotClass+'"></span>';
        html += '<span class="exp-label">'+esc(label)+'</span>';
        html += '<span class="exp-metric">'+metricStr+'</span>';
        html += '<span class="tag '+tagClass+'">'+tagLabel+'</span></div>';
      }
      html += '</div>';
    }

    // Retrieval
    html += '<div class="card-border">';
    html += '<div style="font-size:14px;font-weight:500;margin-bottom:10px">Solution store</div>';
    if (!showSols) {
      html += '<div style="font-size:13px;color:var(--tx2);padding:20px 0;text-align:center">No stored solutions yet</div>';
    }
    for (const s of sols) {
      html += '<div class="sol-card"><div class="sol-header">';
      html += '<span class="sol-problem">'+esc(s.problem)+'</span>';
      html += '<span class="sol-badge">'+s.score.toFixed(2)+'</span></div>';
      html += '<div class="sol-text">'+esc(s.solution)+'</div>';
      if (s.tags && s.tags.length > 0) {
        html += '<div class="sol-tags">';
        for (const tag of s.tags) { html += '<span class="sol-tag">'+esc(tag)+'</span>'; }
        html += '</div>';
      }
      html += '</div>';
    }
    html += '</div>';
    html += '</div>';
  }

  // Reasoning paths — full analysis for each explored branch
  const paths = buildPaths(nodes);
  if (paths.length > 0) {
    html += '<div class="section">Reasoning paths ('+paths.length+' explored)</div>';
    for (let pi = 0; pi < paths.length; pi++) {
      const p = paths[pi];
      const leaf = p.steps[p.steps.length - 1];
      const isSolution = leaf.isTerminal;
      const avgScore = p.steps.reduce((s,n) => s+n.score, 0) / p.steps.length;
      const summaryText = p.steps.slice(1).map(s => s.thought.split(/[.:]/, 1)[0]).join(' → ');

      html += '<div class="path-card">';
      html += '<div class="path-header" onclick="togglePath('+pi+')">';
      html += '<span class="path-rank">#'+(pi+1)+(isSolution?' ★':'')+'</span>';
      html += '<span class="path-summary">'+esc(summaryText)+'</span>';
      html += '<span class="path-score">avg '+avgScore.toFixed(2)+'</span>';
      html += '</div>';
      html += '<div class="path-body" id="path-'+pi+'">';

      const depthLabels = ['Problem', 'Approach', 'Implementation', 'Analysis', 'Detail'];
      for (let si = 0; si < p.steps.length; si++) {
        const step = p.steps[si];
        const label = depthLabels[si] || ('Depth '+si);
        const ev = step.evaluation || 'unexplored';
        html += '<div class="path-step">';
        html += '<div class="path-depth"></div>';
        html += '<div class="path-step-header">';
        html += '<span class="path-step-label">'+label+' (d'+step.depth+')</span>';
        html += '<span class="path-step-score">'+ev+' '+step.score.toFixed(2)+'</span>';
        html += '</div>';
        html += '<div class="path-step-text">'+esc(step.thought)+'</div>';
        html += '</div>';
      }
      html += '</div></div>';
    }
  }

  // Metric chart
  if (exps.length > 1) {
    html += '<div class="section">Metric over experiments</div>';
    html += '<div class="card-border"><div class="chart-wrap"><canvas id="metricChart"></canvas></div></div>';
  }

  app.innerHTML = html;

  // Render chart after DOM is ready
  if (exps.length > 1) {
    renderChart(exps, nodes);
  }
}

function buildTreeSVG(nodes, bestPath) {
  if (!nodes || nodes.length === 0) return '<div style="padding:20px;color:var(--tx2);text-align:center">No nodes yet</div>';

  // Build parent-child map
  const childMap = {};
  const nodeMap = {};
  for (const n of nodes) {
    nodeMap[n.id] = n;
    if (n.parentId) {
      if (!childMap[n.parentId]) childMap[n.parentId] = [];
      childMap[n.parentId].push(n.id);
    }
  }

  // Find root
  const root = nodes.find(n => !n.parentId);
  if (!root) return '';

  // Layout: assign x,y per node (simple layered tree)
  const positions = {};
  const NODE_W = 160, NODE_H = 50, GAP_X = 20, GAP_Y = 80;

  // Count leaves per subtree for width allocation
  function countLeaves(id) {
    const children = childMap[id] || [];
    if (children.length === 0) return 1;
    return children.reduce((s, c) => s + countLeaves(c), 0);
  }

  function layout(id, x, y, widthBudget) {
    const children = childMap[id] || [];
    const cx = x + widthBudget / 2;
    positions[id] = { x: cx, y };

    if (children.length === 0) return;
    const totalLeaves = children.reduce((s, c) => s + countLeaves(c), 0);
    let offset = x;
    for (const cid of children) {
      const cLeaves = countLeaves(cid);
      const cWidth = (cLeaves / totalLeaves) * widthBudget;
      layout(cid, offset, y + GAP_Y + NODE_H, cWidth);
      offset += cWidth;
    }
  }

  const totalLeaves = countLeaves(root.id);
  const svgW = Math.max(680, totalLeaves * (NODE_W + GAP_X));
  const maxDepth = Math.max(...nodes.map(n => n.depth));
  const svgH = (maxDepth + 1) * (NODE_H + GAP_Y) + 60;

  layout(root.id, 20, 20, svgW - 40);

  // Build SVG
  let svg = '<svg class="tree-svg" viewBox="0 0 '+svgW+' '+svgH+'" xmlns="http://www.w3.org/2000/svg">';

  // Edges
  for (const n of nodes) {
    if (n.parentId && positions[n.id] && positions[n.parentId]) {
      const p = positions[n.parentId];
      const c = positions[n.id];
      const isBest = bestPath.has(n.id) && bestPath.has(n.parentId);
      const stroke = isBest ? COLORS.solution : 'var(--tx3)';
      const width = isBest ? 2 : 0.5;
      svg += '<line x1="'+p.x+'" y1="'+(p.y+NODE_H)+'" x2="'+c.x+'" y2="'+c.y+'" stroke="'+stroke+'" stroke-width="'+width+'"/>';
    }
  }

  // Nodes
  for (const n of nodes) {
    const pos = positions[n.id];
    if (!pos) continue;
    const rx = pos.x - NODE_W/2;
    const ry = pos.y;

    let fill, stroke;
    if (n.isTerminal) { fill = '#eaf3de'; stroke = COLORS.solution; }
    else if (n.evaluation === 'sure') { fill = '#e6f1fb'; stroke = COLORS.sure; }
    else if (n.evaluation === 'maybe') { fill = '#faeeda'; stroke = COLORS.maybe; }
    else if (n.evaluation === 'impossible') { fill = '#fcebeb'; stroke = COLORS.impossible; }
    else { fill = 'var(--bg2)'; stroke = 'var(--bd)'; }

    const isDark = matchMedia('(prefers-color-scheme:dark)').matches;
    if (isDark) {
      if (n.isTerminal) fill = '#27500a';
      else if (n.evaluation === 'sure') fill = '#0c447c';
      else if (n.evaluation === 'maybe') fill = '#633806';
      else if (n.evaluation === 'impossible') fill = '#501313';
    }

    const dashed = (!n.evaluation && !n.isTerminal && n.depth > 0) ? ' stroke-dasharray="4 4"' : '';
    svg += '<g class="node-rect">';
    svg += '<rect x="'+rx+'" y="'+ry+'" width="'+NODE_W+'" height="'+NODE_H+'" rx="6" fill="'+fill+'" stroke="'+stroke+'" stroke-width="'+(bestPath.has(n.id)?1.5:0.5)+'"'+dashed+'/>';
    svg += '<text x="'+pos.x+'" y="'+(ry+18)+'" text-anchor="middle" font-size="12" font-weight="500" fill="var(--tx)">'+esc(trunc(n.thought,20))+'</text>';
    if (n.score > 0) {
      svg += '<text x="'+pos.x+'" y="'+(ry+36)+'" text-anchor="middle" font-size="11" fill="var(--tx2)">'+n.score.toFixed(3)+'</text>';
    }
    svg += '</g>';
  }

  svg += '</svg>';
  return '<div style="overflow-x:auto">'+svg+'</div>';
}

function renderChart(exps, nodes) {
  const canvas = document.getElementById('metricChart');
  if (!canvas) return;

  const labels = exps.map(e => trunc(getNodeThought(nodes, e.nodeId), 15));
  const values = exps.map(e => e.metric || null);
  const colors = exps.map(e => {
    if (e.status === 'crashed' || e.status === 'timeout') return COLORS.impossible;
    if (e.kept) return COLORS.sure;
    return COLORS.maybe;
  });

  const validVals = values.filter(v => v !== null);
  const minV = Math.min(...validVals);
  const maxV = Math.max(...validVals);
  const pad = (maxV - minV) * 0.3 || 0.01;

  new Chart(canvas, {
    type: 'bar',
    data: {
      labels,
      datasets: [{ data: values.map(v => v || 0), backgroundColor: colors, borderRadius: 4, barPercentage: 0.7 }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx) => values[ctx.dataIndex] === null ? 'Crashed' : 'metric: ' + values[ctx.dataIndex].toFixed(4)
          }
        }
      },
      scales: {
        y: { min: minV - pad, max: maxV + pad, grid: { color: 'rgba(128,128,128,0.1)' }, ticks: { font: { size: 11 } } },
        x: { ticks: { maxRotation: 45, font: { size: 10 } }, grid: { display: false } }
      }
    }
  });
}

function metricCard(label, val, sub) {
  return '<div class="card"><div class="metric-label">'+label+'</div><div class="metric-val">'+val+'</div><div class="metric-sub">'+esc(sub)+'</div></div>';
}

function getNodeThought(nodes, nodeId) {
  const n = (nodes||[]).find(x => x.id === nodeId);
  return n ? n.thought : nodeId.slice(0,8);
}

function esc(s) { return (s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
function trunc(s, n) { return (s||'').length <= n ? (s||'') : (s||'').slice(0,n)+'...'; }

function buildPaths(nodes) {
  if (!nodes || nodes.length === 0) return [];
  const byId = {}; nodes.forEach(n => byId[n.id] = n);
  const childIds = new Set(); nodes.forEach(n => { if (n.parentId) childIds.add(n.parentId); });
  // Leaves: nodes with no children, not impossible
  const leaves = nodes.filter(n => !childIds.has(n.id) && n.evaluation !== 'impossible');
  // Build path from each leaf to root
  const paths = leaves.map(leaf => {
    const steps = [];
    let cur = leaf;
    while (cur) { steps.unshift(cur); cur = cur.parentId ? byId[cur.parentId] : null; }
    const avg = steps.reduce((s,n) => s+n.score, 0) / steps.length;
    return { steps, avgScore: avg, isSolution: leaf.isTerminal };
  });
  // Sort: solutions first, then by avg score desc
  paths.sort((a,b) => {
    if (a.isSolution !== b.isSolution) return a.isSolution ? -1 : 1;
    return b.avgScore - a.avgScore;
  });
  return paths;
}

function togglePath(idx) {
  const el = document.getElementById('path-'+idx);
  if (el) el.classList.toggle('open');
}

window.addEventListener('hashchange', render);
render();
setInterval(render, 10000);
</script>
</body>
</html>`
