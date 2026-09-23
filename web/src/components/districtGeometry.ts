// Authored diagram cells, NOT geographic borders, coordinates, areas or adjacency.
// Geometry contains no indicator/measure data. Replace this adapter when verified
// geographic boundaries become available; values remain keyed by the API district id.
export interface DistrictShape { path:string; label:[number,number]; sceneSlot:string }
export const schematicViewBox='0 0 1040 700';
export const districtGeometry:Record<string,DistrictShape>={
  // Five adjoining ground areas on one authored plane, never separate floating tiles.
  saryarka:{path:'M440 130 647 243 474 342 267 228 Z',label:[452,289],sceneSlot:'saryarka'},
  almaty:{path:'M647 243 980 424 829 510 496 329 Z',label:[754,383],sceneSlot:'almaty'},
  baikonur:{path:'M658 417 829 510 620 629 449 536 Z',label:[704,552],sceneSlot:'baikonur'},
  esil:{path:'M267 228 465 336 278 443 80 335 Z',label:[243,357],sceneSlot:'esil'},
  nura:{path:'M465 336 636 429 449 536 278 443 Z',label:[435,478],sceneSlot:'nura'},
};

// An unknown id can occupy an unused cell without claiming a geographic location.
// Resolve known ids first so a renamed/reordered district never overlaps another.
export function shapesForDistricts(ids:readonly string[]) {
  const known=new Map(Object.entries(districtGeometry));
  const used=new Set(ids.map(id=>known.get(id)).filter(Boolean));
  const unused=Object.values(districtGeometry).filter(shape=>!used.has(shape));
  let next=0;
  return new Map(ids.map(id=>[id,known.get(id)??unused[next++]] as const));
}
