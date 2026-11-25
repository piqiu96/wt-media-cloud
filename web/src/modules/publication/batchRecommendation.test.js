import {it,expect} from 'vitest';
import {recommendCategoryQuantities} from './batchRecommendation.js';
import {emptyBatch,batchInput} from './batchPreparation.js';
it('distributes within measured capacities while respecting the requested amount',()=>{
 expect(recommendCategoryQuantities([{category:'a',capacity:2},{category:'b',capacity:8}],7)).toEqual([{category:'a',count:2},{category:'b',count:5}]);
 expect(recommendCategoryQuantities([{category:'a',capacity:0}],10)).toEqual([{category:'a',count:0}]);
});
it('restores the local recommendation hint without sending it as business input',()=>{
 const form=emptyBatch({...emptyBatch(),recommendation_quantity:20});
 expect(form.recommendation_quantity).toBe(20);
 expect(batchInput(form)).not.toHaveProperty('recommendation_quantity');
});
