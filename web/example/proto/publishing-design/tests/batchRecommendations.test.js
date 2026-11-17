import { describe, it, expect } from 'vitest';
import { recommendCategoryQuantities } from '../batchRecommendations.js';
const capacities = [{category:'甲',capacity:2},{category:'乙',capacity:5},{category:'丙',capacity:0}];
describe('advisory batch target recommendation', () => {
  it('uses available capacity when no overall target is given', () => {
    expect(recommendCategoryQuantities(capacities, null)).toEqual([{category:'甲',count:2},{category:'乙',count:5},{category:'丙',count:0}]);
  });
  it('shares the requested quantity, respecting category order and capacity', () => {
    expect(recommendCategoryQuantities(capacities, 5)).toEqual([{category:'甲',count:2},{category:'乙',count:3},{category:'丙',count:0}]);
    expect(recommendCategoryQuantities(capacities, 1).map(t=>t.count)).toEqual([1,0,0]);
  });
  it('returns partial recommendations without rejecting an unreachable target', () => {
    expect(recommendCategoryQuantities(capacities, 9999).map(t=>t.count)).toEqual([2,5,0]);
  });
  it('does not invent resources for unavailable or empty categories', () => {
    expect(recommendCategoryQuantities([], 10)).toEqual([]);
    expect(recommendCategoryQuantities([{category:'甲',capacity:0}],10)).toEqual([{category:'甲',count:0}]);
  });
});
