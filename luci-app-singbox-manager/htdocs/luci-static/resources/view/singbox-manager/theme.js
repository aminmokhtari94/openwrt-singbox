'use strict';
'require baseclass';

// Shared visual theme for every SingBox Manager view. The stylesheet is the
// single source of truth for the card-based look modelled on the dashboard; it
// is injected once into <head> (keyed by id) so every page renders identically
// and re-renders (poll / refreshView) never duplicate it.
var CSS = [
	/* ---- page + section cards ---- */
	'.singbox-manager-page{display:grid;gap:16px}',
	'.singbox-manager-page.narrow{max-width:960px}',
	'.singbox-manager-dashboard{display:grid;gap:16px}',
	'.singbox-manager-live{display:grid;gap:16px}',
	'.singbox-manager-section{display:grid;gap:12px;border:1px solid var(--border-color-medium);border-radius:12px;padding:16px 20px;background:var(--background-color-high)}',
	'.singbox-manager-section>h3{margin:0;font-size:15px;font-weight:600}',
	'.singbox-manager-section-header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}',
	'.singbox-manager-section-header h3{margin:0;font-size:15px;font-weight:600}',
	'.singbox-manager-hint{margin:0;font-size:12px;color:var(--text-color-medium)}',
	'.singbox-manager-inline-help{margin:0;font-size:12px;color:var(--text-color-medium)}',

	/* ---- badges / status ---- */
	'.singbox-manager-badge{font-size:11px;font-weight:600;padding:3px 10px;border-radius:999px;background:var(--border-color-medium);color:var(--text-color-medium);white-space:nowrap}',
	'.singbox-manager-badge.on{background:rgba(15,122,57,.15);color:#0f7a39}',
	'.singbox-manager-badge.off{background:rgba(180,35,24,.12);color:#b42318}',
	'.singbox-manager-badge.warn{background:rgba(180,83,9,.14);color:#b45309}',
	'.singbox-manager-status-ok{color:#0f7a39;font-weight:600}',
	'.singbox-manager-status-error{color:#b42318;font-weight:600}',

	/* ---- controls ---- */
	'.singbox-manager-controls{display:flex;gap:18px;flex-wrap:wrap}',
	'.singbox-manager-inline-control{display:flex;align-items:center;gap:10px;font-size:13px}',
	'.singbox-manager-filters{display:flex;gap:8px;flex-wrap:wrap}',
	'.singbox-manager-actions{display:flex;gap:6px;flex-wrap:wrap;align-items:center}',
	'.singbox-manager-toolbar{display:flex;gap:8px;flex-wrap:wrap}',

	/* ---- definition grid (label / value) ---- */
	'.singbox-manager-grid{display:grid;grid-template-columns:minmax(130px,190px) minmax(0,1fr);gap:10px 14px;align-items:start}',
	'.singbox-manager-label{font-size:12px;color:var(--text-color-medium);font-weight:600}',

	/* ---- setting rows (toggle list) ---- */
	'.singbox-manager-rows{display:grid;gap:0}',
	'.singbox-manager-row{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:12px 0;border-top:1px solid var(--border-color-medium)}',
	'.singbox-manager-row:first-child{border-top:0}',
	'.singbox-manager-row-label{font-size:13px;font-weight:600}',
	'.singbox-manager-row-hint{font-size:11px;color:var(--text-color-medium);margin-top:2px}',
	'.singbox-manager-row-control{flex:0 0 auto;display:flex;align-items:center}',
	'.singbox-manager-row-control input.cbi-input-text{min-width:min(280px,46vw)}',
	'.singbox-manager-meta{display:flex;gap:16px;flex-wrap:wrap;font-size:11px;color:var(--text-color-medium);padding-top:4px}',

	/* ---- toggle switch ---- */
	'.singbox-manager-toggle{position:relative;display:inline-block;width:42px;height:24px;flex:0 0 auto}',
	'.singbox-manager-toggle input{position:absolute;opacity:0;width:0;height:0}',
	'.singbox-manager-toggle span{position:absolute;inset:0;cursor:pointer;background:var(--border-color-medium);border-radius:24px;transition:.2s}',
	'.singbox-manager-toggle span:before{content:"";position:absolute;height:18px;width:18px;left:3px;top:3px;background:#fff;border-radius:50%;transition:.2s}',
	'.singbox-manager-toggle input:checked+span{background:#2271b1}',
	'.singbox-manager-toggle input:checked+span:before{transform:translateX(18px)}',

	/* ---- tables ---- */
	'.singbox-manager-table{width:100%;border-collapse:collapse;font-size:13px}',
	'.singbox-manager-table th,.singbox-manager-table td{padding:9px 10px;text-align:left;vertical-align:middle}',
	'.singbox-manager-table thead th{font-size:11px;text-transform:uppercase;letter-spacing:.04em;color:var(--text-color-medium);font-weight:600;border-bottom:2px solid var(--border-color-medium);white-space:nowrap}',
	'.singbox-manager-table tbody td{border-bottom:1px solid var(--border-color-medium)}',
	'.singbox-manager-table tbody tr:last-child td{border-bottom:0}',
	'.singbox-manager-table tbody tr:hover td{background:var(--background-color-low)}',
	'.singbox-manager-table tbody tr:not(.singbox-manager-group-row) td[colspan]{text-align:center;color:var(--text-color-medium);padding:18px 10px}',
	'.singbox-manager-table th:last-child,.singbox-manager-table td:last-child{text-align:right}',
	'.singbox-manager-table .singbox-manager-actions{justify-content:flex-end}',
	'.singbox-manager-group-row td{background:var(--background-color-low);font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.04em;color:var(--text-color-medium);text-align:left!important}',

	/* ---- modal forms ---- */
	'.singbox-manager-modal-form{display:grid;gap:12px;min-width:min(520px,90vw)}',
	'.singbox-manager-modal-form.wide{grid-template-columns:repeat(auto-fit,minmax(180px,1fr));min-width:min(760px,90vw)}',
	'.singbox-manager-modal-form label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
	'.singbox-manager-check{display:flex!important;align-items:center;gap:8px}',
	'.singbox-manager-modal-form.wide .singbox-manager-check{margin-top:20px}',
	'.singbox-manager-import-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}',
	'.singbox-manager-import-grid label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
	'.singbox-manager-import-link{grid-column:1/-1}',
	'.singbox-manager-import-link textarea{min-height:96px;resize:vertical}',
	'.singbox-manager-import-help{grid-column:1/-1;margin:0;font-size:12px;color:var(--text-color-medium)}',

	/* ---- device picker chips ---- */
	'.singbox-manager-devicepicker{display:grid;gap:6px;margin-top:-4px}',
	'.singbox-manager-devicepicker-label{font-size:11px;color:var(--text-color-medium)}',
	'.singbox-manager-chips{display:flex;gap:6px;flex-wrap:wrap}',
	'.singbox-manager-chip{font-size:11px;padding:2px 8px!important;border-radius:12px}',

	/* ---- alerts + preformatted blocks ---- */
	'.singbox-manager-warning{padding:10px 12px;border-radius:8px;border-left:4px solid #b45309;background:rgba(180,83,9,.10);font-size:13px}',
	'.singbox-manager-preview{box-sizing:border-box;max-width:100%;min-height:60px;overflow:auto;padding:12px;border:1px solid var(--border-color-medium);border-radius:8px;background:var(--background-color-low);font:12px/1.5 monospace;white-space:pre}',
	'.singbox-manager-log{box-sizing:border-box;max-width:100%;min-height:360px;overflow:auto;padding:12px;border:1px solid var(--border-color-medium);border-radius:8px;background:var(--background-color-low);font:12px/1.5 monospace;white-space:pre-wrap;word-break:break-word}',
	'.singbox-manager-details{border:1px solid var(--border-color-medium);border-radius:8px;padding:0}',
	'.singbox-manager-details>summary{cursor:pointer;padding:10px 12px;font-size:13px;font-weight:600;color:var(--text-color-medium);list-style:none}',
	'.singbox-manager-details[open]>summary{border-bottom:1px solid var(--border-color-medium)}',
	'.singbox-manager-details>.singbox-manager-grid{padding:12px}',

	/* ---- dashboard hero ---- */
	'.singbox-manager-hero{display:flex;align-items:center;gap:20px;flex-wrap:wrap;border:1px solid var(--border-color-medium);border-radius:12px;padding:16px 20px;background:var(--background-color-high)}',
	'.singbox-manager-hero-status{display:flex;align-items:center;gap:14px;min-width:160px}',
	'.singbox-manager-dot{width:14px;height:14px;border-radius:50%;background:#9aa0a6;flex:0 0 auto;box-shadow:0 0 0 4px rgba(154,160,166,.18)}',
	'.singbox-manager-dot.on{background:#0f7a39;box-shadow:0 0 0 4px rgba(15,122,57,.18)}',
	'.singbox-manager-dot.idle{background:#d68b00;box-shadow:0 0 0 4px rgba(214,139,0,.18)}',
	'.singbox-manager-dot.off{background:#b42318;box-shadow:0 0 0 4px rgba(180,35,24,.18)}',
	'.singbox-manager-hero-state{font-size:22px;font-weight:700;line-height:1.1}',
	'.singbox-manager-hero-sub{font-size:12px;color:var(--text-color-medium)}',
	'.singbox-manager-hero-facts{display:grid;grid-template-columns:repeat(auto-fit,minmax(120px,1fr));gap:12px 20px;flex:1 1 320px}',
	'.singbox-manager-hero-facts>div{display:grid;gap:3px}',
	'.singbox-manager-hero-facts span{font-size:11px;color:var(--text-color-medium)}',
	'.singbox-manager-hero-facts strong{font-size:14px;overflow-wrap:anywhere}',
	'.singbox-manager-hero-facts strong.ok{color:#0f7a39}',
	'.singbox-manager-hero-facts strong.error{color:#b42318}',
	'.singbox-manager-hero-action{flex:0 0 auto}',
	'.singbox-manager-hero-action .btn{font-size:15px;padding:8px 26px}',

	/* ---- dashboard charts ---- */
	'.singbox-manager-charts{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:12px}',
	'.singbox-manager-chart{border:1px solid var(--border-color-medium);border-radius:12px;padding:14px;background:var(--background-color-high);display:grid;gap:8px}',
	'.singbox-manager-chart-head{display:flex;align-items:baseline;justify-content:space-between}',
	'.singbox-manager-chart-title{font-size:12px;color:var(--text-color-medium);font-weight:600}',
	'.singbox-manager-chart-rate{font-size:16px;font-weight:700}',
	'.singbox-manager-spark{height:72px}',
	'.singbox-manager-spark-empty{display:flex;align-items:center;justify-content:center;height:100%;font-size:12px;color:var(--text-color-medium)}',
	'.singbox-manager-chart-foot{font-size:11px;color:var(--text-color-medium)}',

	/* ---- dashboard metrics ---- */
	'.singbox-manager-metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px}',
	'.singbox-manager-metric{border:1px solid var(--border-color-medium);border-radius:12px;padding:14px;background:var(--background-color-high)}',
	'.singbox-manager-metric-label{font-size:12px;color:var(--text-color-medium);margin-bottom:6px}',
	'.singbox-manager-metric-value{font-size:18px;font-weight:600;overflow-wrap:anywhere}',
	'.singbox-manager-metric-value.ok{color:#0f7a39}',
	'.singbox-manager-metric-value.error{color:#b42318}',
	'.singbox-manager-logs{display:grid;gap:12px}',

	/* ---- responsive ---- */
	'@media(max-width:760px){',
	'.singbox-manager-grid{grid-template-columns:1fr}',
	'.singbox-manager-table{display:block;overflow-x:auto;white-space:nowrap}',
	'.singbox-manager-preview{white-space:pre-wrap;word-break:break-word}',
	'.singbox-manager-row{flex-direction:column;align-items:stretch}',
	'.singbox-manager-row-control{justify-content:flex-start}',
	'.singbox-manager-section{padding:14px}',
	'}'
].join('');

return baseclass.extend({
	// inject adds the shared stylesheet to <head> once. Re-renders are common
	// (polling, refreshView replacing the page subtree); the id guard keeps a
	// single <style> element regardless of how often render() runs.
	inject: function() {
		if (document.getElementById('singbox-manager-theme'))
			return;
		var style = document.createElement('style');
		style.id = 'singbox-manager-theme';
		style.textContent = CSS;
		document.head.appendChild(style);
	}
});
