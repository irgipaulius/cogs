import React, { useEffect, useMemo, useRef, useState } from 'react';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';

const COUNTRIES_GEOJSON =
  'https://raw.githubusercontent.com/johan/world.geo.json/master/countries.geo.json';
const ISO_MAP_URL =
  'https://raw.githubusercontent.com/lukes/ISO-3166-Countries-with-Regional-Codes/master/slim-3/slim-3.json';

const regionNames = new Intl.DisplayNames(['en'], { type: 'region' });

const BLACK_STYLE = {
  version: 8,
  name: 'cogs-black',
  sources: {},
  layers: [
    {
      id: 'background',
      type: 'background',
      paint: { 'background-color': '#030304' },
    },
  ],
};

function buildAlpha3ToAlpha2(rows) {
  const map = {};
  for (const row of rows || []) {
    const a2 = row['alpha-2'];
    const a3 = row['alpha-3'];
    if (a2 && a3) map[a3] = a2;
  }
  return map;
}

function enrichGeoJSON(base, counts, a3ToA2) {
  const features = base.features.map((f) => {
    const iso3 = f.id || f.properties?.id;
    const iso2 = a3ToA2[iso3] || iso3;
    const n = counts[iso2] || counts[iso3] || 0;
    return {
      ...f,
      properties: {
        ...f.properties,
        iso2,
        iso3,
        req_count: n,
      },
    };
  });
  return { type: 'FeatureCollection', features };
}

function fillColorExpr(mode) {
  const hue = mode === 'write' ? [251, 191, 36] : [94, 234, 212];
  const c = (a) => `rgba(${hue[0]}, ${hue[1]}, ${hue[2]}, ${a})`;
  return [
    'interpolate',
    ['linear'],
    ['get', 'req_count'],
    0,
    'rgba(255,255,255,0.04)',
    1,
    c(0.65),
    5,
    c(0.82),
    25,
    c(0.95),
  ];
}

export function GeoMap({ statsRead, statsWrite, mode }) {
  const containerRef = useRef(null);
  const mapRef = useRef(null);
  const popupRef = useRef(null);
  const geoBaseRef = useRef(null);
  const a3ToA2Ref = useRef({});
  const hoverIso3Ref = useRef('');
  const [mapReady, setMapReady] = useState(false);
  const [loadErr, setLoadErr] = useState('');

  const counts = useMemo(() => {
    if (mode === 'read') return statsRead || {};
    if (mode === 'write') return statsWrite || {};
    const out = { ...(statsRead || {}) };
    for (const [k, v] of Object.entries(statsWrite || {})) {
      out[k] = (out[k] || 0) + v;
    }
    return out;
  }, [mode, statsRead, statsWrite]);

  const maxCount = useMemo(() => Math.max(1, ...Object.values(counts)), [counts]);

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    const map = new maplibregl.Map({
      container: containerRef.current,
      style: BLACK_STYLE,
      center: [-3.7, 40.2],
      zoom: 1.6,
      minZoom: 0.8,
      maxZoom: 8,
      attributionControl: false,
    });
    mapRef.current = map;
    popupRef.current = new maplibregl.Popup({
      closeButton: false,
      closeOnClick: false,
      className: 'geo-popup',
      offset: 12,
    });
    map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');
    map.addControl(
      new maplibregl.AttributionControl({ compact: true, customAttribution: '© OSM · CoGS' }),
      'bottom-right',
    );

    map.on('load', () => {
      Promise.all([fetch(COUNTRIES_GEOJSON).then((r) => r.json()), fetch(ISO_MAP_URL).then((r) => r.json())])
        .then(([geo, isoRows]) => {
          a3ToA2Ref.current = buildAlpha3ToAlpha2(isoRows);
          geoBaseRef.current = geo;
          const enriched = enrichGeoJSON(geo, counts, a3ToA2Ref.current);

          map.addSource('countries', { type: 'geojson', data: enriched });

          map.addLayer({
            id: 'countries-fill',
            type: 'fill',
            source: 'countries',
            paint: {
              'fill-color': fillColorExpr(mode),
              'fill-opacity': 0.92,
              'fill-outline-color': 'rgba(255,255,255,0.08)',
            },
          });

          map.addLayer({
            id: 'countries-line',
            type: 'line',
            source: 'countries',
            paint: {
              'line-color': 'rgba(255,255,255,0.14)',
              'line-width': 0.5,
            },
          });

          map.addLayer({
            id: 'countries-hover',
            type: 'line',
            source: 'countries',
            paint: {
              'line-color': mode === 'write' ? '#fcd34d' : '#5eead4',
              'line-width': 2.2,
            },
            filter: ['==', ['get', 'iso3'], ''],
          });

          setMapReady(true);
          setLoadErr('');
        })
        .catch((err) => {
          setMapReady(false);
          setLoadErr(String(err.message || err));
        });
    });

    return () => {
      popupRef.current?.remove();
      map.remove();
      mapRef.current = null;
      geoBaseRef.current = null;
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    const base = geoBaseRef.current;
    if (!map || !base || !mapReady) return;
    const src = map.getSource('countries');
    if (!src) return;
    src.setData(enrichGeoJSON(base, counts, a3ToA2Ref.current));
    if (map.getLayer('countries-fill')) {
      map.setPaintProperty('countries-fill', 'fill-color', fillColorExpr(mode));
    }
    if (map.getLayer('countries-hover')) {
      map.setPaintProperty('countries-hover', 'line-color', mode === 'write' ? '#fcd34d' : '#5eead4');
    }
  }, [counts, mode, mapReady]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !mapReady) return;

    const onMove = (e) => {
      const feats = map.queryRenderedFeatures(e.point, { layers: ['countries-fill'] });
      if (!feats.length) {
        map.getCanvas().style.cursor = '';
        hoverIso3Ref.current = '';
        if (map.getLayer('countries-hover')) {
          map.setFilter('countries-hover', ['==', ['get', 'iso3'], '']);
        }
        popupRef.current?.remove();
        return;
      }

      const f = feats[0].properties || {};
      const iso2 = f.iso2 || '';
      const iso3 = f.iso3 || '';
      const n = f.req_count || 0;
      const name = f.name || (iso2 ? regionNames.of(iso2) : '') || iso3 || 'Unknown';
      const read = statsRead?.[iso2] || 0;
      const write = statsWrite?.[iso2] || 0;

      map.getCanvas().style.cursor = 'pointer';
      if (hoverIso3Ref.current !== iso3) {
        hoverIso3Ref.current = iso3;
        if (map.getLayer('countries-hover')) {
          map.setFilter('countries-hover', ['==', ['get', 'iso3'], iso3]);
        }
      }

      popupRef.current
        .setLngLat(e.lngLat)
        .setHTML(
          `<strong>${name}</strong> <span class="geo-code">${iso2}</span><br/>` +
            `Reads: ${read} · Writes: ${write}<br/>` +
            `<span class="geo-muted">View: ${n} (${mode})</span>`,
        )
        .addTo(map);
    };

    const onLeave = () => {
      map.getCanvas().style.cursor = '';
      hoverIso3Ref.current = '';
      if (map.getLayer('countries-hover')) {
        map.setFilter('countries-hover', ['==', ['get', 'iso3'], '']);
      }
      popupRef.current?.remove();
    };

    map.on('mousemove', onMove);
    map.on('mouseleave', onLeave);
    return () => {
      map.off('mousemove', onMove);
      map.off('mouseleave', onLeave);
    };
  }, [mapReady, statsRead, statsWrite, mode]);

  return (
    <div className="geo-map-wrap">
      <div ref={containerRef} className="geo-map" role="img" aria-label="World traffic map" />
      {!mapReady && !loadErr && <div className="geo-map-loading muted">Loading map…</div>}
      {loadErr && <div className="geo-map-loading err">Map failed: {loadErr}</div>}
      <div className="geo-legend muted">
        Black basemap · brighter = more traffic · max {maxCount} ({mode === 'write' ? 'writes' : mode === 'read' ? 'reads' : 'all'})
      </div>
    </div>
  );
}
