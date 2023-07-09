package db

import (
	"context"
	"github.com/aalug/go-gin-job-search/utils"
	"github.com/stretchr/testify/require"
	"testing"
)

// createRandomJobSkill create and return a random job skill
func createRandomJobSkill(t *testing.T, job *Job, skill string) JobSkill {
	var params CreateJobSkillParams
	if skill == "" {
		params.Skill = utils.RandomString(6)
	} else {
		params.Skill = skill
	}

	if job == nil {
		j := createRandomJob(t, nil, jobDetails{})
		params.JobID = j.ID
	} else {
		params.JobID = job.ID
	}

	jobSkill, err := testQueries.CreateJobSkill(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, jobSkill)
	require.Equal(t, jobSkill.JobID, params.JobID)
	require.Equal(t, jobSkill.Skill, params.Skill)
	require.NotZero(t, jobSkill.ID)

	return jobSkill
}

func TestQueries_CreateJobSkillCreateJobSkill(t *testing.T) {
	createRandomJobSkill(t, nil, "")
}

func TestQueries_DeleteJobSkill(t *testing.T) {
	jobSkill := createRandomJobSkill(t, nil, "")
	err := testQueries.DeleteJobSkill(context.Background(), jobSkill.ID)
	require.NoError(t, err)
}

func TestQueries_ListJobSkillsByJobID(t *testing.T) {
	job := createRandomJob(t, nil, jobDetails{})
	job2 := createRandomJob(t, nil, jobDetails{})
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			createRandomJobSkill(t, &job, "")
		} else {
			createRandomJobSkill(t, &job2, "")
		}
	}
	params := ListJobSkillsByJobIDParams{
		JobID:  job.ID,
		Limit:  5,
		Offset: 0,
	}

	jobSkills, err := testQueries.ListJobSkillsByJobID(context.Background(), params)
	require.NoError(t, err)
	require.Len(t, jobSkills, 5)
	for _, jobSkill := range jobSkills {
		require.NotEmpty(t, jobSkill)
	}
}

func TestQueries_ListJobsBySkill(t *testing.T) {
	skill := "testSkill"
	for i := 0; i < 10; i++ {
		createRandomJob(t, nil, jobDetails{})
		if i%2 == 0 {
			createRandomJobSkill(t, nil, skill)
		}
	}

	params := ListJobsBySkillParams{
		Skill:  skill,
		Limit:  5,
		Offset: 0,
	}

	jobIDs, err := testQueries.ListJobsBySkill(context.Background(), params)
	require.NoError(t, err)
	require.Len(t, jobIDs, 5)
	for _, jobID := range jobIDs {
		require.NotZero(t, jobID)
	}
}

func TestQueries_UpdateJobSkill(t *testing.T) {
	jobSkill := createRandomJobSkill(t, nil, "")
	params := UpdateJobSkillParams{
		ID:    jobSkill.ID,
		Skill: utils.RandomString(6),
	}

	jobSkill, err := testQueries.UpdateJobSkill(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, jobSkill)
	require.Equal(t, jobSkill.ID, params.ID)
	require.Equal(t, jobSkill.Skill, params.Skill)
}
