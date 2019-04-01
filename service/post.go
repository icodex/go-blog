/**
 * Created by GoLand.
 * User: xzghua@gmail.com
 * Date: 2019-01-12
 * Time: 21:03
 */
package service

import (
	"github.com/izghua/go-blog/common"
	"github.com/izghua/go-blog/conf"
	"github.com/izghua/go-blog/entity"
	"github.com/izghua/zgh"
	"github.com/microcosm-cc/bluemonday"
	"gopkg.in/russross/blackfriday.v2"
)

func ConsolePostCount(limit int,offset int) (count int64,err error) {
	post := new(entity.ZPosts)
	count,err = conf.SqlServer.Desc("id").Limit(limit,offset).Count(post)
	if err != nil {
		zgh.ZLog().Error("message","service.ConsolePostCount",err,err.Error())
		return 0,err
	}
	return count,nil
}


func ConsolePostIndex(limit int,offset int) (postListArr []*common.ConsolePostList,err error) {
	post := new(entity.ZPosts)
	rows,err := conf.SqlServer.Desc("id").Limit(limit,offset).Rows(post)

	if err != nil {
		zgh.ZLog().Error("message","service.ConsolePostIndex",err,err.Error())
		return nil,err
	}

	defer rows.Close()
	for rows.Next() {
		//post
		post := new(entity.ZPosts)
		err = rows.Scan(post)
		if err != nil {
			zgh.ZLog().Error("message","service.ConsolePostIndex",err,err.Error())
			return nil,err
		}

		consolePost := common.ConsolePost{
			Id: post.Id,
			Uid: post.Uid,
			Title: post.Title,
			Summary: post.Summary,
			Original: post.Original,
			Content: post.Content,
			Password: post.Password,
			CreatedAt: post.CreatedAt,
			UpdatedAt: post.UpdatedAt,
		}

		//category
		cates,err := GetPostCateByPostId(post.Id)
		if err != nil {
			zgh.ZLog().Error("message","service.ConsolePostIndex",err,err.Error())
			return nil,err
		}
		consoleCate := common.ConsoleCate{
			Id: cates.Id,
			Name: cates.Name,
			DisplayName: cates.DisplayName,
			SeoDesc: cates.SeoDesc,
		}

		//tag
		tagIds,err := GetPostTagsByPostId(post.Id)
		if err != nil {
			zgh.ZLog().Error("message","service.ConsolePostIndex",err,err.Error())
			return nil,err
		}
		tags,err := GetTagsByIds(tagIds)
		if err != nil {
			zgh.ZLog().Error("message","service.ConsolePostIndex",err,err.Error())
			return nil,err
		}
		var consoleTags []common.ConsoleTag
		for _,v := range tags {
			consoleTag := common.ConsoleTag{
				Id: v.Id,
				Name: v.Name,
				DisplayName: v.DisplayName,
				SeoDesc: v.SeoDesc,
				Num: v.Num,
			}
			consoleTags = append(consoleTags,consoleTag)
		}


		//view
		view,err := PostView(post.Id)
		if err != nil {
			zgh.ZLog().Error("message","service.ConsolePostIndex",err,err.Error())
			return nil,err
		}
		consoleView := common.ConsoleView{
			Num: view.Num,
		}

		//user
		user,err := GetUserById(post.UserId)
		if err != nil {
			zgh.ZLog().Error("message","service.ConsolePostIndex",err,err.Error())
			return nil,err
		}
		consoleUser := common.ConsoleUser{
			Id: user.Id,
			Name: user.Name,
			Email: user.Email,
			Status: user.Status,
		}

		postList := common.ConsolePostList{
			Post: consolePost,
			Category: consoleCate,
			Tags: consoleTags,
			View: consoleView,
			Author: consoleUser,
		}
		postListArr = append(postListArr,&postList)
	}

	return postListArr,nil
}

func PostView(postId int) (*entity.ZPostViews,error) {
	postV := new(entity.ZPostViews)
	_,err := conf.SqlServer.Where("post_id = ?",postId).Cols("num").Get(postV)
	if err != nil {
		zgh.ZLog().Error("message","service.PostView",err,err.Error())
	}
	return postV,nil
}


func PostStore(ps common.PostStore) {
	postCreate := &entity.ZPosts{
		Title: ps.Title,
		UserId: 1,
		Summary: ps.Summary,
		Original: ps.Content,
	}

	unsafe := blackfriday.Run([]byte(ps.Content))
	html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	postCreate.Content = string(html)


	session := conf.SqlServer.NewSession()
	defer session.Close()
	affected,err := session.Insert(postCreate)
	if err != nil {
		zgh.ZLog().Error("message","service.PostStore",err,err.Error())
		_ = session.Rollback()
		return
	}
	if affected < 1 {
		zgh.ZLog().Error("message","service.PostStore","err","post store no succeed")
		_ = session.Rollback()
		return
	}



	if ps.Category > 0 {
		postCateCreate := entity.ZPostCate{
			PostId: postCreate.Id,
			CateId: ps.Category,
		}
		affected,err := session.Insert(postCateCreate)
		if err != nil {
			zgh.ZLog().Error("message","service.PostStore",err,err.Error())
			_ = session.Rollback()
			return
		}

		if affected < 1 {
			zgh.ZLog().Error("message","service.PostStore","err","post cate store no succeed")
			_ = session.Rollback()
			return
		}
	}

	if len(ps.Tags) > 0 {
		for _,v := range ps.Tags {
			postTagCreate := entity.ZPostTag{
				PostId: postCreate.Id,
				TagId: v,
			}
			affected,err := session.Insert(postTagCreate)
			if err != nil {
				zgh.ZLog().Error("message","service.PostStore post tag insert err",err,err.Error())
				_ = session.Rollback()
				return
			}
			if affected < 1 {
				zgh.ZLog().Error("message","service.PostStore","err","post cate store no succeed")
				_ = session.Rollback()
				return
			}
		}
	}

	postView := entity.ZPostViews{
		PostId: postCreate.Id,
		Num: 1,
	}

	affected,err = session.Insert(postView)
	if err != nil {
		zgh.ZLog().Error("message","service.PostStore",err,err.Error())
		_ = session.Rollback()
		return
	}

	if affected < 1 {
		zgh.ZLog().Error("message","service.PostStore","err","post view store no succeed")
		_ = session.Rollback()
		return
	}

	_ = session.Commit()

	uid,err := conf.ZHashId.Encode([]int{postCreate.Id})
	if err != nil {
		zgh.ZLog().Error("message","service.PostStore create uid error",err,err.Error())
		return
	}

	newPostCreate := entity.ZPosts{
		Uid:uid,
	}
	affected,err = session.Where("id = ?",postCreate.Id).Update(newPostCreate)
	if err != nil {
		zgh.ZLog().Error("message","service.PostStore",err,err.Error())
		return
	}

	if affected < 1 {
		zgh.ZLog().Error("message","service.PostStore","err","post view store no succeed")
		return
	}

	return
}

func PostDetail(postId int) (p *entity.ZPosts,err error) {
	post := new(entity.ZPosts)
	_,err = conf.SqlServer.Where("id = ?",postId).Get(post)
	if err != nil {
		zgh.ZLog().Error("message","service.PostEdit",err,err.Error())
		return  post,err
	}
	return post,nil
}

func PostIdTag(postId int) (tagIds []int,err error) {
	postTag := make([]entity.ZPostTag,0)
	err = conf.SqlServer.Where("post_id = ?",postId).Find(&postTag)
	if err != nil {
		zgh.ZLog().Error("message","service.PostIdTag",err,err.Error())
		return
	}

	for _,v := range postTag {
		tagIds = append(tagIds,v.TagId)
	}
	return tagIds,nil
}

func PostCate(postId int) (int,error) {
	postCate := new(entity.ZPostCate)
	_,err := conf.SqlServer.Where("post_id = ?",postId).Get(postCate)
	if err != nil {
		zgh.ZLog().Error("message","service.PostCate",err,err.Error())
		return 0,err
	}
	return postCate.CateId,nil
}


func PostUpdate(postId int,ps common.PostStore) {
	postUpdate := &entity.ZPosts{
		Title: ps.Title,
		UserId: 1,
		Summary: ps.Summary,
		Original: ps.Content,
	}

	unsafe := blackfriday.Run([]byte(ps.Content))
	html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
	postUpdate.Content = string(html)


	session := conf.SqlServer.NewSession()
	defer session.Close()
	affected,err := session.Where("id = ?",postId).Update(postUpdate)
	if err != nil {
		zgh.ZLog().Error("message","service.PostUpdate",err,err.Error())
		_ = session.Rollback()
		return
	}
	if affected < 1 {
		zgh.ZLog().Error("message","service.PostUpdate","err","post update no succeed")
		_ = session.Rollback()
		return
	}

	postCate := new(entity.ZPostCate)
	_,err = session.Where("post_id = ?",postId).Delete(postCate)
	if err != nil {
		zgh.ZLog().Error("message","service.PostUpdate","err","post cate delete no succeed")
		_ = session.Rollback()
		return
	}

	if ps.Category > 0 {
		postCateCreate := entity.ZPostCate{
			PostId: postId,
			CateId: ps.Category,
		}

		affected,err := session.Insert(postCateCreate)
		if err != nil {
			zgh.ZLog().Error("message","service.PostUpdate",err,err.Error())
			_ = session.Rollback()
			return
		}

		if affected < 1 {
			zgh.ZLog().Error("message","service.PostUpdate","err","post cate update no succeed")
			_ = session.Rollback()
			return
		}
	}

	postTag := new(entity.ZPostTag)
	_,err = session.Where("post_id = ?",postId).Delete(postTag)

	if err != nil {
		zgh.ZLog().Error("message","service.PostUpdate","err","post tag delete no succeed")
		_ = session.Rollback()
		return
	}

	if len(ps.Tags) > 0 {
		for _,v := range ps.Tags {
			postTagCreate := entity.ZPostTag{
				PostId: postId,
				TagId: v,
			}
			affected,err := session.Insert(postTagCreate)
			if err != nil {
				zgh.ZLog().Error("message","service.PostUpdate post tag insert err",err,err.Error())
				_ = session.Rollback()
				return
			}
			if affected < 1 {
				zgh.ZLog().Error("message","service.PostUpdate","err","post cate update no succeed")
				_ = session.Rollback()
				return
			}
		}
	}
	_ = session.Commit()

	return
}