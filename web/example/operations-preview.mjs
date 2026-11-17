export function createOperationsMiddleware(){
 return (req,res,next)=>{
   const url=new URL(req.url,'http://localhost');
   if(req.method==='GET'||req.method==='HEAD'){
     if(/^\/example\/operations(?:\/|$)/.test(url.pathname)){
       req.url='/example/index.html';
     }
   }
   next();
 };
}
