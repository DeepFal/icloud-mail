import assert from "node:assert/strict";
import test from "node:test";
import { normalizeMessage, messagePath, getMessages, getMessage, patchMessage } from "../src/workspace/mail-api.js";

test("workspace normalization respects archived content and sender address contract", () => {
 const mail = normalizeMessage({id:4,alias_id:2,mailbox_uid:9,from:[{name:"Sender",email:"sender@example.test"}],content_state:"missing",read:false,starred:true});
 assert.equal(mail.aliasId,2);assert.equal(mail.mailboxUid,9);assert.equal(mail.from[0].address,"sender@example.test");assert.equal(mail.contentState,"missing");assert.equal(mail.read,false);assert.equal(normalizeMessage({}).contentState,"metadata_only");
});
test("workspace download paths stay inside authenticated message endpoints", () => {
 assert.equal(messagePath("/mailboxes/2/messages/9/attachments/3"),"/admin/api/v1/mailboxes/2/messages/9/attachments/3");assert.equal(messagePath("/mailboxes/2/messages/9/source"),"/admin/api/v1/mailboxes/2/messages/9/source");
 for(const value of ["https://elsewhere.test/x","//elsewhere.test/x","/mailboxes/../auth/logout","/mailboxes/2/messages/9/source?x=1","/mailboxes/2/messages/9/attachments/0",""]){assert.equal(messagePath(value),"");}
});
test("workspace list/detail/state use envelopes, exact aliases and CSRF", async(t) => {
 const calls=[];t.mock.method(globalThis,"fetch",async(url,options)=>{calls.push({url,options});const data=options.method==="PATCH"?{id:4,alias_id:2,mailbox_uid:9,read:true}:String(url).includes("/mailboxes/")?{message:{id:4,alias_id:2,mailbox_uid:9},text:"完整正文",html:"<p>正文</p>",content_available:true,attachments:[]}:{items:[{id:4,alias_id:2,mailbox_uid:9}],total:1,unread_count:1,has_more:false};return new Response(JSON.stringify({data}),{status:200,headers:{"content-type":"application/json"}});});
 const list=await getMessages({aliasId:2,unread:true,limit:40});assert.equal(list.items.length,1);assert.equal(list.unreadCount,1);assert.match(calls[0].url,/alias_id=2/);assert.match(calls[0].url,/unread=1/);
 const detail=await getMessage(2,9);assert.equal(detail.text,"完整正文");assert.equal(detail.html,"<p>正文</p>");assert.equal(detail.contentAvailable,true);
 const updated=await patchMessage(2,9,{read:true},"csrf-fixture");assert.equal(updated.read,true);assert.equal(calls[2].options.headers.get("X-CSRF-Token"),"csrf-fixture");assert.equal(calls[2].options.body,'{"read":true}');
});
