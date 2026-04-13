package dashboard

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>ToT Dashboard</title>
<script src="https://cdnjs.cloudflare.com/ajax/libs/Chart.js/4.4.1/chart.umd.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/d3/7.9.0/d3.min.js"></script>
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
.radial-tree-wrap{position:relative;overflow:hidden;border-radius:var(--radius)}
.radial-tree-wrap svg{display:block;width:100%;cursor:grab}
.radial-tree-wrap svg:active{cursor:grabbing}
.radial-tree-wrap .link{fill:none;stroke:var(--tx3);stroke-width:1}
.radial-tree-wrap .link-best{stroke:var(--green);stroke-width:2.5}
.radial-tree-wrap .node-circle{cursor:pointer;stroke-width:2;transition:r .2s}
.radial-tree-wrap .node-circle:hover{r:9}
.radial-tree-wrap .node-label{font-size:11px;fill:var(--tx);pointer-events:none}
.radial-tree-wrap .node-score{font-size:9px;font-family:var(--mono);fill:var(--tx2);pointer-events:none}
.node-panel{position:fixed;top:0;right:-420px;width:420px;height:100vh;background:var(--bg);border-left:1px solid var(--bd);box-shadow:-4px 0 20px rgba(0,0,0,.1);z-index:100;transition:right .3s ease;overflow-y:auto;padding:24px}
.node-panel.open{right:0}
.node-panel-close{position:absolute;top:16px;right:16px;font-size:20px;cursor:pointer;color:var(--tx2);background:none;border:none;line-height:1}
.node-panel-close:hover{color:var(--tx)}
.node-panel h2{font-size:16px;font-weight:600;margin-bottom:16px;padding-right:32px}
.node-panel-path-step{padding:12px 0;border-bottom:1px solid var(--bd)}
.node-panel-path-step:last-child{border-bottom:none}
.node-panel-depth{font-size:10px;font-weight:600;text-transform:uppercase;letter-spacing:0.5px;color:var(--tx2);margin-bottom:4px}
.node-panel-eval{display:inline-block;font-size:10px;padding:1px 6px;border-radius:4px;margin-left:8px;font-weight:500}
.node-panel-thought{font-size:13px;line-height:1.65;color:var(--tx);white-space:pre-wrap;word-break:break-word;margin-top:6px}
.node-panel-current{background:var(--bg2);border-radius:var(--radius);padding:12px;margin:-4px -4px -4px -4px}
.tree-svg{width:100%;overflow-x:auto}
.node-rect{cursor:pointer;transition:opacity .15s}
.node-rect:hover{opacity:0.85}
.legend{display:flex;gap:14px;margin-bottom:10px;font-size:12px;color:var(--tx2);flex-wrap:wrap}
.legend span{display:flex;align-items:center;gap:4px}
.legend i{width:10px;height:10px;border-radius:2px;display:inline-block}
.chart-wrap{position:relative;height:220px;margin-top:8px}
#app{min-height:300px}
.empty{text-align:center;padding:60px 20px;color:var(--tx2);font-size:14px}
.header-row{display:flex;justify-content:space-between;align-items:flex-start;margin-bottom:4px}
.btn{font-size:13px;font-weight:500;padding:8px 16px;border-radius:var(--radius);border:none;cursor:pointer;transition:background .15s}
.btn-primary{background:var(--blue);color:#fff}
.btn-primary:hover{opacity:0.9}
.btn-ghost{background:transparent;color:var(--tx2);border:0.5px solid var(--bd)}
.btn-ghost:hover{background:var(--bg2)}
.create-form{border:0.5px solid var(--bd);border-radius:var(--radius);padding:16px;margin-bottom:16px;display:none}
.create-form.open{display:block}
.create-form input,.create-form select{font-family:var(--sans);font-size:14px;padding:8px 12px;border:0.5px solid var(--bd);border-radius:var(--radius);background:var(--bg);color:var(--tx);width:100%;box-sizing:border-box}
.create-form input:focus,.create-form select:focus{outline:none;border-color:var(--blue)}
.create-form label{font-size:12px;color:var(--tx2);display:block;margin-bottom:4px;margin-top:12px}
.create-form label:first-child{margin-top:0}
.create-form-actions{display:flex;gap:8px;margin-top:16px;justify-content:flex-end}
.create-form .error{font-size:12px;color:var(--red);margin-top:8px;display:none}
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
  let html = '<div class="header-row"><div><h1>Tree of Thoughts</h1><p class="subtitle">All reasoning trees</p></div>';
  html += '<button class="btn btn-primary" onclick="toggleCreateForm()">+ New Tree</button></div>';
  html += '<div class="create-form" id="create-form">';
  html += '<label>Problem statement</label>';
  html += '<input type="text" id="cf-problem" placeholder="What problem should the tree explore?">';
  html += '<label>Search strategy</label>';
  html += '<select id="cf-strategy"><option value="beam">Beam search</option><option value="bfs">Breadth-first</option><option value="dfs">Depth-first</option></select>';
  html += '<div class="error" id="cf-error"></div>';
  html += '<div class="create-form-actions">';
  html += '<button class="btn btn-ghost" onclick="toggleCreateForm()">Cancel</button>';
  html += '<button class="btn btn-primary" onclick="submitCreateTree()">Create</button>';
  html += '</div></div>';
  if (!trees || trees.length === 0) {
    html += '<div class="empty"><p>No reasoning trees yet. Create one above.</p></div>';
  } else {
    for (const t of trees) {
      html += '<div class="tree-list-item" onclick="location.hash=\'tree/'+t.id+'\'">';
      html += '<div><div class="tree-problem">'+esc(t.problem)+'</div>';
      html += '<div class="tree-meta"><span>'+t.nodeCount+' nodes</span><span>'+t.experimentCount+' experiments</span><span>'+t.strategy+'</span></div></div>';
      html += '<span class="tag '+(t.status==='active'?'tag-keep':'tag-discard')+'">'+t.status+'</span>';
      html += '</div>';
    }
  }
  app.innerHTML = html;
}

function toggleCreateForm() {
  const form = document.getElementById('create-form');
  if (form) form.classList.toggle('open');
}

async function submitCreateTree() {
  const problem = document.getElementById('cf-problem').value.trim();
  const strategy = document.getElementById('cf-strategy').value;
  const errEl = document.getElementById('cf-error');
  if (!problem) {
    errEl.textContent = 'Please enter a problem statement.';
    errEl.style.display = 'block';
    return;
  }
  errEl.style.display = 'none';
  try {
    const res = await fetch('/api/trees', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({problem, strategy})
    });
    const data = await res.json();
    if (!res.ok) {
      errEl.textContent = data.error || 'Failed to create tree.';
      errEl.style.display = 'block';
      return;
    }
    location.hash = 'tree/' + data.id;
  } catch (e) {
    errEl.textContent = 'Network error: ' + e.message;
    errEl.style.display = 'block';
  }
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

  // Tree visualization — D3 radial tree
  html += '<div class="section">Reasoning tree — click any node to explore its path</div>';
  html += '<div class="card-border">';
  html += '<div class="legend">';
  html += '<span><i style="background:'+COLORS.sure+'"></i> Sure</span>';
  html += '<span><i style="background:'+COLORS.maybe+'"></i> Maybe</span>';
  html += '<span><i style="background:'+COLORS.impossible+'"></i> Impossible</span>';
  html += '<span><i style="background:'+COLORS.solution+'"></i> Solution</span>';
  html += '<span><i style="background:'+COLORS.unexplored+'"></i> Unexplored</span>';
  html += '</div>';
  html += '<div class="radial-tree-wrap" id="radial-tree"></div>';
  html += '</div>';
  html += '<div class="node-panel" id="node-panel"></div>';

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

  // Render D3 radial tree after DOM is ready
  if (nodes && nodes.length > 0) {
    renderRadialTree(nodes, bestPath);
  }

  // Render chart after DOM is ready
  if (exps.length > 1) {
    renderChart(exps, nodes);
  }
}

function renderRadialTree(nodes, bestPath) {
  const container = document.getElementById('radial-tree');
  if (!container || !nodes || nodes.length === 0) {
    if (container) container.innerHTML = '<div style="padding:20px;color:var(--tx2);text-align:center">No nodes yet</div>';
    return;
  }

  // Build hierarchy data for D3
  const byId = {}; nodes.forEach(n => byId[n.id] = n);
  const root = nodes.find(n => !n.parentId);
  if (!root) return;

  function buildHierarchy(node) {
    const children = nodes.filter(n => n.parentId === node.id);
    const result = { ...node, children: children.map(c => buildHierarchy(c)) };
    if (result.children.length === 0) delete result.children;
    return result;
  }

  const data = buildHierarchy(root);
  const hierRoot = d3.hierarchy(data);

  // Sizing
  const nodeCount = hierRoot.descendants().length;
  const radius = Math.max(200, Math.min(380, nodeCount * 28));
  const w = radius * 2 + 160;
  const h = w;

  container.innerHTML = '';
  const svg = d3.select(container).append('svg')
    .attr('viewBox', [-w/2, -h/2, w, h].join(' '))
    .attr('width', '100%')
    .style('min-height', '420px')
    .style('max-height', '640px');

  // Enable pan + zoom
  const g = svg.append('g');
  svg.call(d3.zoom().scaleExtent([0.3, 3]).on('zoom', (e) => g.attr('transform', e.transform)));

  // Radial tree layout
  const tree = d3.tree()
    .size([2 * Math.PI, radius])
    .separation((a, b) => (a.parent === b.parent ? 1 : 2) / a.depth || 1);
  tree(hierRoot);

  // Radial point helper
  function radialPoint(x, y) {
    return [(y = +y) * Math.cos(x -= Math.PI / 2), y * Math.sin(x)];
  }

  // Draw links
  g.append('g').selectAll('path')
    .data(hierRoot.links())
    .join('path')
    .attr('class', d => {
      const sid = d.source.data.id, tid = d.target.data.id;
      return bestPath.has(sid) && bestPath.has(tid) ? 'link link-best' : 'link';
    })
    .attr('d', d3.linkRadial().angle(d => d.x).radius(d => d.y));

  // Draw non-root nodes
  const branchNodes = hierRoot.descendants().filter(d => d.depth > 0);
  const nodeG = g.append('g').selectAll('g')
    .data(branchNodes)
    .join('g')
    .attr('transform', d => 'translate(' + radialPoint(d.x, d.y).join(',') + ')');

  // Node circles
  nodeG.append('circle')
    .attr('class', 'node-circle')
    .attr('r', d => d.data.isTerminal ? 9 : 6)
    .attr('fill', d => {
      if (d.data.isTerminal) return COLORS.solution;
      if (d.data.evaluation === 'sure') return COLORS.sure;
      if (d.data.evaluation === 'maybe') return COLORS.maybe;
      if (d.data.evaluation === 'impossible') return COLORS.impossible;
      return COLORS.unexplored;
    })
    .attr('stroke', d => bestPath.has(d.data.id) ? COLORS.solution : 'var(--bg)')
    .attr('stroke-width', d => bestPath.has(d.data.id) ? 3 : 2)
    .on('click', (event, d) => {
      event.stopPropagation();
      showNodePanel(d.data, byId);
    });

  // Node labels (first phrase, radially rotated)
  nodeG.append('text')
    .attr('class', 'node-label')
    .attr('dy', '0.31em')
    .attr('x', d => d.x < Math.PI === !d.children ? 8 : -8)
    .attr('text-anchor', d => d.x < Math.PI === !d.children ? 'start' : 'end')
    .attr('transform', d => {
      const angle = d.x * 180 / Math.PI;
      return 'rotate(' + (angle < 180 ? angle - 90 : angle + 90) + ')';
    })
    .text(d => {
      const t = d.data.thought || '';
      const first = t.split(/[.:(\n]/, 1)[0];
      return trunc(first, 28);
    });

  // Score labels on branches
  nodeG.filter(d => d.data.score > 0)
    .append('text')
    .attr('class', 'node-score')
    .attr('dy', '-10')
    .attr('text-anchor', 'middle')
    .text(d => d.data.score.toFixed(2));

  // Root node — special centered treatment
  const rootG = g.append('g').attr('class', 'root-node');
  rootG.append('circle')
    .attr('class', 'node-circle')
    .attr('r', 22)
    .attr('fill', COLORS.sure)
    .attr('stroke', 'var(--bg)')
    .attr('stroke-width', 3)
    .attr('cx', 0).attr('cy', 0)
    .style('cursor', 'pointer')
    .on('click', (event) => {
      event.stopPropagation();
      showNodePanel(hierRoot.data, byId);
    });

  // Root icon (small tree symbol)
  rootG.append('text')
    .attr('text-anchor', 'middle')
    .attr('dy', '0.35em')
    .attr('fill', 'white')
    .attr('font-size', '16px')
    .style('pointer-events', 'none')
    .text('\u25CE');

  // Root label — wrapped problem text below the circle
  const rootText = hierRoot.data.thought || '';
  const words = rootText.split(/\s+/);
  const lines = []; let line = '';
  for (const w of words) {
    if ((line + ' ' + w).trim().length > 40) { lines.push(line.trim()); line = w; }
    else { line = (line + ' ' + w).trim(); }
  }
  if (line) lines.push(line);
  const maxLines = Math.min(lines.length, 3);

  for (let i = 0; i < maxLines; i++) {
    rootG.append('text')
      .attr('text-anchor', 'middle')
      .attr('y', 32 + i * 14)
      .attr('fill', 'var(--tx)')
      .attr('font-size', '11px')
      .attr('font-weight', i === 0 ? '600' : '400')
      .style('pointer-events', 'none')
      .text(i === maxLines - 1 && lines.length > maxLines ? lines[i] + '...' : lines[i]);
  }

  // Click background to close panel
  svg.on('click', () => closeNodePanel());
}

function showNodePanel(node, byId) {
  const panel = document.getElementById('node-panel');
  if (!panel) return;

  // Walk path from this node to root
  const steps = [];
  let cur = node;
  while (cur) {
    steps.unshift(cur);
    cur = cur.parentId ? byId[cur.parentId] : null;
  }

  const depthLabels = ['Problem', 'Approach', 'Implementation', 'Analysis', 'Detail'];
  const evalColors = { sure: COLORS.sure, maybe: COLORS.maybe, impossible: COLORS.impossible };

  let html = '<button class="node-panel-close" onclick="closeNodePanel()">&times;</button>';
  html += '<h2>Path to: ' + esc(steps[steps.length-1].thought.split(/[.:]/,1)[0]) + '</h2>';

  for (let i = 0; i < steps.length; i++) {
    const s = steps[i];
    const label = depthLabels[i] || ('Depth ' + i);
    const ev = s.evaluation || 'unexplored';
    const isCurrent = (s.id === node.id);
    const evColor = evalColors[ev] || 'var(--tx3)';

    html += '<div class="node-panel-path-step">';
    html += '<div class="node-panel-depth">' + label + ' (d' + s.depth + ')';
    html += '<span class="node-panel-eval" style="background:' + evColor + '22;color:' + evColor + '">' + ev + ' ' + s.score.toFixed(2) + '</span>';
    if (s.isTerminal) html += '<span class="node-panel-eval" style="background:' + COLORS.solution + '22;color:' + COLORS.solution + '">solution</span>';
    html += '</div>';
    html += '<div class="node-panel-thought' + (isCurrent ? ' node-panel-current' : '') + '">' + esc(s.thought) + '</div>';
    html += '</div>';
  }

  panel.innerHTML = html;
  panel.classList.add('open');
}

function closeNodePanel() {
  const panel = document.getElementById('node-panel');
  if (panel) panel.classList.remove('open');
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
// Live updates via SSE, fallback to polling
let _t; function dr(){clearTimeout(_t);_t=setTimeout(render,100);}
if (typeof EventSource !== 'undefined') {
  const es = new EventSource('/api/events');
  ['tree.created','tree.status_changed','tree.auto_paused','tree.linked',
   'solution.marked','solution.compacted','solution.linked','url.ingested'
  ].forEach(e => es.addEventListener(e, dr));
  ['thought.added','thought.evaluated','subtree.pruned',
   'experiment.prepared','experiment.completed','experiment.failed',
   'solution.stored'
  ].forEach(e => es.addEventListener(e, () => { if (location.hash.startsWith('#tree/')) dr(); else dr(); }));
  es.onerror = () => {
    es.close();
    setInterval(render, 10000);
  };
} else {
  setInterval(render, 10000);
}
</script>
</body>
</html>`
